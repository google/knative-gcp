/*
Copyright 2019 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auditlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/events/auditlogs/resources"
	pubsubreconciler "github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	"cloud.google.com/go/logging/logadmin"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "cloudauditlogssources.events.cloud.google.com"
	publisherRole = "roles/pubsub.publisher"
)

type Reconciler struct {
	*pubsubreconciler.PubSubBase

	auditLogsSourceLister  listers.CloudAuditLogsSourceLister
	logadminClientProvider glogadmin.CreateFn
	pubsubClientProvider   gpubsub.CreateFn
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CloudAuditLogsSource resource
// with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the CloudAuditLogsSource resource with this namespace/name
	original, err := c.auditLogsSourceLister.CloudAuditLogsSources(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The CloudAuditLogsSource resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("CloudAuditLogsSource in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	s := original.DeepCopy()

	reconcileErr := c.reconcile(ctx, s)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		s.Status.ObservedGeneration = s.Generation
	}

	if _, updated, err := c.updateFinalizers(ctx, s); err != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update CloudAuditLogsSource finalizers", zap.Error(err))
		c.Recorder.Eventf(s, corev1.EventTypeWarning, "UpdateFailed", "Failed to update finalizers for CloudAuditLogsSource %q: %v", s.Name, err)
		return err
	} else if updated {
		c.Recorder.Eventf(s, corev1.EventTypeNormal, "Updated", "Updated CloudAuditLogsSource %q finalizers", s.Name)
	}

	// If we didn't change anything then don't call updateStatus.
	// This is important because the copy we loaded from the
	// informer's cache may be stale and we don't want to
	// overwrite a prior update to status with this stale state.
	if !equality.Semantic.DeepEqual(original.Status, s.Status) {
		if err := c.updateStatus(ctx, original, s); err != nil {
			c.Logger.Warn("Failed to update CloudAuditLogsSource status", zap.Error(err))
			c.Recorder.Eventf(s, corev1.EventTypeWarning, "UpdateFailed",
				"Failed to update status for CloudAuditLogsSource %q: %v", s.Name, err)
			return err
		} else if reconcileErr == nil {
			c.Recorder.Eventf(s, corev1.EventTypeNormal, "Updated", "Updated CloudAuditLogsSource %q", s.Name)
		}
	}

	if reconcileErr != nil {
		c.Recorder.Event(s, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, s *v1alpha1.CloudAuditLogsSource) error {
	ctx = logging.WithLogger(ctx, c.Logger.With(zap.Any("auditlogsource", s)))

	s.Status.InitializeConditions()

	// See if the source has been deleted.
	if s.DeletionTimestamp != nil {
		if err := c.deleteSink(ctx, s); err != nil {
			s.Status.MarkSinkNotReady("SinkDeleteFailed", "Failed to delete Stackdriver sink: %s", err.Error())
			return err
		}
		s.Status.MarkSinkNotReady("SinkDeleted", "Successfully deleted Stackdriver sink: %s", s.Status.StackdriverSink)

		if err := c.PubSubBase.DeletePubSub(ctx, s); err != nil {
			return err
		}
		s.Status.StackdriverSink = ""
		c.removeFinalizer(s)
		return nil
	}

	// Ensure the finalizer's there, since we're about to attempt
	// to change external state with the topic, so we need to
	// clean it up.
	c.addFinalizer(s)
	topic := resources.GenerateTopicName(s)
	t, ps, err := c.PubSubBase.ReconcilePubSub(ctx, s, topic, resourceGroup)
	if err != nil {
		return err
	}
	c.Logger.Debugf("Reconciled: PubSub: %+v PullSubscription: %+v", t, ps)

	sink, err := c.reconcileSink(ctx, s)
	if err != nil {
		return err
	}
	s.Status.StackdriverSink = sink
	s.Status.MarkSinkReady()
	c.Logger.Debugf("Reconciled Stackdriver sink: %+v", sink)

	return nil
}

func (c *Reconciler) reconcileSink(ctx context.Context, s *v1alpha1.CloudAuditLogsSource) (string, error) {
	sink, err := c.ensureSinkCreated(ctx, s)
	if err != nil {
		s.Status.MarkSinkNotReady("SinkCreateFailed", "failed to ensure creation of logging sink: %v", err)
		return "", err
	}
	err = c.ensureSinkIsPublisher(ctx, s, sink)
	if err != nil {
		s.Status.MarkSinkNotReady("SinkNotPublisher", "failed to ensure sink has pubsub.publisher permission on source topic: %v", err)
		return "", err
	}
	return sink.ID, nil
}

func (c *Reconciler) ensureSinkCreated(ctx context.Context, s *v1alpha1.CloudAuditLogsSource) (*logadmin.Sink, error) {
	sinkID := s.Status.StackdriverSink
	if sinkID == "" {
		sinkID = resources.GenerateSinkName(s)
	}
	logadminClient, err := c.logadminClientProvider(ctx, s.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create LogAdmin client", zap.Error(err))
		return nil, err
	}
	sink, err := logadminClient.Sink(ctx, sinkID)
	if status.Code(err) == codes.NotFound {
		filterBuilder := resources.FilterBuilder{}
		filterBuilder.WithServiceName(s.Spec.ServiceName).WithMethodName(s.Spec.MethodName)
		if s.Spec.ResourceName != "" {
			filterBuilder.WithResourceName(s.Spec.ResourceName)
		}
		sink = &logadmin.Sink{
			ID:          sinkID,
			Destination: resources.GenerateTopicResourceName(s),
			Filter:      filterBuilder.GetFilterQuery(),
		}
		sink, err = logadminClient.CreateSinkOpt(ctx, sink, logadmin.SinkOptions{UniqueWriterIdentity: true})
		// Handle AlreadyExists in-case of a race between another create call.
		if status.Code(err) == codes.AlreadyExists {
			sink, err = logadminClient.Sink(ctx, sinkID)
		}
	}
	return sink, err
}

// Ensures that the sink has been granted the pubsub.publisher role on the source topic.
func (c *Reconciler) ensureSinkIsPublisher(ctx context.Context, s *v1alpha1.CloudAuditLogsSource, sink *logadmin.Sink) error {
	pubsubClient, err := c.pubsubClientProvider(ctx, s.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create PubSub client", zap.Error(err))
		return err
	}
	topicIam := pubsubClient.Topic(s.Status.TopicID).IAM()
	topicPolicy, err := topicIam.Policy(ctx)
	if err != nil {
		return err
	}
	if !topicPolicy.HasRole(sink.WriterIdentity, publisherRole) {
		topicPolicy.Add(sink.WriterIdentity, publisherRole)
		if err = topicIam.SetPolicy(ctx, topicPolicy); err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Debug(
			"Granted the Stackdriver Sink writer identity roles/pubsub.publisher on PubSub Topic.",
			zap.String("writerIdentity", sink.WriterIdentity),
			zap.String("topicID", s.Status.TopicID))
	}
	return nil
}

// deleteSink looks at status.SinkID and if non-empty will delete the
// previously created stackdriver sink.
func (c *Reconciler) deleteSink(ctx context.Context, s *v1alpha1.CloudAuditLogsSource) error {
	if s.Status.StackdriverSink == "" {
		return nil
	}
	logadminClient, err := c.logadminClientProvider(ctx, s.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create LogAdmin client", zap.Error(err))
		return err
	}
	if err = logadminClient.DeleteSink(ctx, s.Status.StackdriverSink); status.Code(err) != codes.NotFound {
		return err
	}
	return nil
}

func (c *Reconciler) addFinalizer(s *v1alpha1.CloudAuditLogsSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (c *Reconciler) removeFinalizer(s *v1alpha1.CloudAuditLogsSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (c *Reconciler) updateStatus(ctx context.Context, original *v1alpha1.CloudAuditLogsSource, desired *v1alpha1.CloudAuditLogsSource) error {
	existing := original.DeepCopy()
	return reconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		// The first iteration tries to use the informer's state, subsequent attempts fetch the latest state via API.
		if attempts > 0 {
			existing, err = c.RunClientSet.EventsV1alpha1().CloudAuditLogsSources(desired.Namespace).Get(desired.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		// If there's nothing to update, just return.
		if equality.Semantic.DeepEqual(existing.Status, desired.Status) {
			return nil
		}
		becomesReady := desired.Status.IsReady() && !existing.Status.IsReady()

		existing.Status = desired.Status
		_, err = c.RunClientSet.EventsV1alpha1().CloudAuditLogsSources(desired.Namespace).UpdateStatus(existing)

		if err == nil && becomesReady {
			duration := time.Since(existing.ObjectMeta.CreationTimestamp.Time)
			logging.FromContext(ctx).Desugar().Info("CloudAuditLogsSource became ready", zap.Any("after", duration))
			c.Recorder.Event(existing, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("CloudAuditLogsSource %q became ready", existing.Name))
			if err := c.StatsReporter.ReportReady("CloudAuditLogsSource", existing.Namespace, existing.Name, duration); err != nil {
				logging.FromContext(ctx).Infof("failed to record ready for CloudAuditLogsSource, %v", err)
			}
		}

		return err
	})
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (r *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.CloudAuditLogsSource) (*v1alpha1.CloudAuditLogsSource, bool, error) {
	s, err := r.auditLogsSourceLister.CloudAuditLogsSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := s.DeepCopy()

	var finalizers []string

	// If there's nothing to update, just return.
	existingFinalizers := sets.NewString(existing.Finalizers...)
	desiredFinalizers := sets.NewString(desired.Finalizers...)

	if desiredFinalizers.Has(finalizerName) {
		if existingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Add the finalizer.
		finalizers = append(existing.Finalizers, finalizerName)
	} else {
		if !existingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Remove the finalizer.
		existingFinalizers.Delete(finalizerName)
		finalizers = existingFinalizers.List()
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      finalizers,
			"resourceVersion": existing.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return desired, false, err
	}

	update, err := r.RunClientSet.EventsV1alpha1().CloudAuditLogsSources(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}
