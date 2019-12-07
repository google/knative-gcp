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

package cloudauditlog

import (
	"context"
	"fmt"
	"time"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsubreconciler "github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	"cloud.google.com/go/logging/logadmin"
	"cloud.google.com/go/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "stackdrivers.events.cloud.google.com"
)

type Reconciler struct {
	*pubsubreconciler.PubSubBase

	cloudauditlogLister listers.CloudAuditLogLister
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CloudAuditLog resource
// with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CloudAuditLog resource with this namespace/name
	original, err := c.cloudauditlogLister.CloudAuditLogs(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The CloudAuditLog resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("CloudAuditLog '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	cal := original.DeepCopy()

	reconcileErr := c.reconcile(ctx, cal)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		cal.Status.ObservedGeneration = cal.Generation
	}

	// If we didn't change anything then don't call updateStatus.
	// This is important because the copy we loaded from the
	// informer's cache may be stale and we don't want to
	// overwrite a prior update to status with this stale state.
	if !equality.Semantic.DeepEqual(original.Status, cal.Status) {
		if _, err := c.updateStatus(ctx, cal); err != nil {
			c.Logger.Warn("Failed to update CloudAuditLog Source status", zap.Error(err))
			c.Recorder.Eventf(cal, corev1.EventTypeWarning, "UpdateFailed",
				"Failed to update status for CloudAuditLog %q: %v", cal.Name, err)
			return err
		} else if reconcileErr == nil {
			c.Recorder.Eventf(cal, corev1.EventTypeNormal, "Updated", "Updated CloudAuditLog %q", cal.Name)
		}
	}

	if reconcileErr != nil {
		c.Recorder.Event(cal, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, cal *v1alpha1.CloudAuditLog) error {
	topic := cal.Status.TopicID
	if topic == "" {
		topic = fmt.Sprintf("cloudauditlog-%s", string(cal.UID))
	}
	cal.Status.InitializeConditions()

	// See if the source has been deleted.
	if cal.DeletionTimestamp != nil {
		err := c.deleteSink(ctx, cal)
		if err != nil {
			cal.Status.MarkSinkNotReady("SinkDeleteFailed", "Failed to delete Stackdriver sink: %s", err.Error())
			return fmt.Errorf("failed to delete Stackdriver sink: %v", err)
		}
		cal.Status.MarkSinkNotReady("SinkDeleted", "Successfully deleted Stackdriver sink: %s", cal.Status.SinkID)
		cal.Status.SinkID = ""

		err = c.PubSubBase.DeletePubSub(ctx, cal)
		if err != nil {
			return fmt.Errorf("failed to delete pubsub resources: %s", err)
		}
		c.removeFinalizer(cal)
		return nil
	}

	// Ensure the finalizer's there, since we're about to attempt
	// to change external state with the topic, so we need to
	// clean it up.
	c.ensureFinalizer(cal)
	t, ps, err := c.PubSubBase.ReconcilePubSub(ctx, cal, topic, resourceGroup)
	if err != nil {
		return err
	}
	c.Logger.Infof("Reconciled: PubSub: %+v PullSubscription: %+v", t, ps)

	sink, err := c.reconcileSink(ctx, cal)
	if err != nil {
		return err
	}
	cal.Status.SinkID = sink
	cal.Status.MarkSinkReady()
	c.Logger.Infof("Reconciled Stackdriver sink: %+v", sink)

	return nil
}

func (c *Reconciler) reconcileSink(ctx context.Context, cal *v1alpha1.CloudAuditLog) (string, error) {
	sink, err := c.ensureSinkCreated(ctx, cal)
	if err != nil {
		cal.Status.MarkSinkNotReady("SinkCreateFailed", "failed to ensure creation of logging sink: %v", err)
		return "", err
	}
	err = c.ensureSinkIsPublisher(ctx, cal, sink)
	if err != nil {
		cal.Status.MarkSinkNotReady("SinkNotReady", "failed to ensure sink has pubsub.publisher permission on source topic: %v", err)
		return "", err
	}
	return sink.ID, nil
}

func (c *Reconciler) ensureSinkCreated(ctx context.Context, cal *v1alpha1.CloudAuditLog) (*logadmin.Sink, error) {
	sinkID := cal.Status.SinkID
	if sinkID == "" {
		sinkID = fmt.Sprintf("sink-%s", string(cal.UID))
	}
	logadminClient, err := logadmin.NewClient(ctx, cal.Status.ProjectID)
	if err != nil {
		return nil, err
	}
	sink, err := logadminClient.Sink(ctx, sinkID)
	if status.Code(err) == codes.NotFound {
		sink = &logadmin.Sink{
			ID:          sinkID,
			Destination: fmt.Sprintf("pubsub.googleapis.com/projects/%s/topics/%s", cal.Status.ProjectID, cal.Status.TopicID),
			//Filter:
		}
		sink, err = logadminClient.CreateSinkOpt(ctx, sink, logadmin.SinkOptions{UniqueWriterIdentity: true})
		if status.Code(err) == codes.AlreadyExists {
			sink, err = logadminClient.Sink(ctx, sinkID)
		}
	}
	return sink, err
}

// Ensures that the sink has been granted the pubsub.publisher role on the source topic.
func (c *Reconciler) ensureSinkIsPublisher(ctx context.Context, cal *v1alpha1.CloudAuditLog, sink *logadmin.Sink) error {
	pubsubClient, err := pubsub.NewClient(ctx, cal.Status.ProjectID)
	if err != nil {
		return err
	}
	topicIam := pubsubClient.Topic(cal.Status.TopicID).IAM()
	topicPolicy, err := topicIam.Policy(ctx)
	if err != nil {
		return err
	}
	if !topicPolicy.HasRole(sink.WriterIdentity, "roles/pubsub.publisher") {
		topicPolicy.Add(sink.WriterIdentity, "roles/pubsub.publisher")
		if err = topicIam.SetPolicy(ctx, topicPolicy); err != nil {
			return err
		}
		c.Logger.Infof("Gave writer identify '%s' roles/pubsub.publisher on topic '%s'.", sink.WriterIdentity, cal.Status.TopicID)
	}
	return nil
}

// deleteSink looks at status.SinkID and if non-empty will delete the
// previously created stackdriver sink.
func (c *Reconciler) deleteSink(ctx context.Context, cal *v1alpha1.CloudAuditLog) error {
	if cal.Status.SinkID == "" {
		return nil
	}
	logadminClient, err := logadmin.NewClient(ctx, cal.Status.ProjectID)
	if err != nil {
		return err
	}
	return logadminClient.DeleteSink(ctx, cal.Status.SinkID)
}

func (c *Reconciler) ensureFinalizer(cal *v1alpha1.CloudAuditLog) {
	finalizers := sets.NewString(cal.Finalizers...)
	finalizers.Insert(finalizerName)
	cal.Finalizers = finalizers.List()
}

func (c *Reconciler) removeFinalizer(cal *v1alpha1.CloudAuditLog) {
	finalizers := sets.NewString(cal.Finalizers...)
	finalizers.Delete(finalizerName)
	cal.Finalizers = finalizers.List()
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.CloudAuditLog) (*v1alpha1.CloudAuditLog, error) {
	source, err := c.cloudauditlogLister.CloudAuditLogs(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// Check if there is anything to update.
	if equality.Semantic.DeepEqual(source.Status, desired.Status) {
		return source, nil
	}
	becomesReady := desired.Status.IsReady() && !source.Status.IsReady()

	// Don't modify the informers copy.
	existing := source.DeepCopy()
	existing.Status = desired.Status
	src, err := c.RunClientSet.EventsV1alpha1().CloudAuditLogs(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("CloudAuditLog %q became ready after %v", source.Name, duration)

		if err := c.StatsReporter.ReportReady("CloudAuditLog", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for CloudAuditLog, %v", err)
		}
	}

	return src, err
}
