/*
Copyright 2019 Google LLC

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

package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/knative-gcp/pkg/tracing"
	"github.com/google/knative-gcp/pkg/utils"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	tracingconfig "knative.dev/pkg/tracing/config"

	serving "knative.dev/serving/pkg/apis/serving/v1"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/topic/resources"
	gstatus "google.golang.org/grpc/status"
)

const (
	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for Topic resources.
type Reconciler struct {
	*pubsub.PubSubBase

	// topicLister index properties about topics.
	topicLister listers.TopicLister
	// serviceLister index properties about services.
	serviceLister servinglisters.ServiceLister

	publisherImage string
	tracingConfig  *tracingconfig.Config

	// createClientFn is the function used to create the Pub/Sub client that interacts with Pub/Sub.
	// This is needed so that we can inject a mock client for UTs purposes.
	createClientFn gpubsub.CreateFn
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Topic resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the Topic resource with this namespace/name
	original, err := r.topicLister.Topics(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("Topic in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	topic := original.DeepCopy()

	// Reconcile this copy of the Topic and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = r.reconcile(ctx, topic)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		topic.Status.ObservedGeneration = topic.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, topic.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := r.updateFinalizers(ctx, topic); fErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Topic finalizers", zap.Error(fErr))
		r.Recorder.Eventf(topic, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for Topic %q: %v", topic.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		r.Recorder.Eventf(topic, corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topic.Name)
	}

	if equality.Semantic.DeepEqual(original.Status, topic.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if uErr := r.updateStatus(ctx, original, topic); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Topic status", zap.Error(uErr))
		r.Recorder.Eventf(topic, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Topic %q: %v", topic.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(topic, corev1.EventTypeNormal, "Updated", "Updated Topic %q", topic.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(topic, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, topic *v1alpha1.Topic) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("topic", topic)))

	topic.Status.InitializeConditions()

	if topic.DeletionTimestamp != nil {
		if topic.Spec.PropagationPolicy == v1alpha1.TopicPolicyCreateDelete {
			logging.FromContext(ctx).Desugar().Debug("Deleting Pub/Sub topic")
			if err := r.deleteTopic(ctx, topic); err != nil {
				topic.Status.MarkNoTopic("TopicDeleteFailed", "Failed to delete Pub/Sub topic: %s", err.Error())
				return err
			}
			topic.Status.MarkNoTopic("TopicDeleted", "Successfully deleted Pub/Sub topic: %s", topic.Status.TopicID)
			topic.Status.TopicID = ""
		}
		removeFinalizer(topic)
		return nil
	}

	// Add the finalizer.
	addFinalizer(topic)

	if err := r.reconcileTopic(ctx, topic); err != nil {
		topic.Status.MarkNoTopic("TopicReconcileFailed", "Failed to reconcile Pub/Sub topic: %s", err.Error())
		return err
	}
	topic.Status.MarkTopicReady()
	// Set the topic being used.
	topic.Status.TopicID = topic.Spec.Topic

	err, svc := r.reconcilePublisher(ctx, topic)
	if err != nil {
		topic.Status.MarkPublisherNotDeployed("PublisherReconcileFailed", "Failed to reconcile Publisher: %s", err.Error())
		return err
	}

	// Update the topic.
	topic.Status.PropagatePublisherStatus(&svc.Status)
	if svc.Status.IsReady() {
		topic.Status.SetAddress(svc.Status.Address.URL)
	}

	return nil
}

func (r *Reconciler) reconcileTopic(ctx context.Context, topic *v1alpha1.Topic) error {
	if topic.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(topic.Spec.Project)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return err
		}
		// Set the projectID in the status.
		topic.Status.ProjectID = projectID
	}

	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	client, err := r.createClientFn(ctx, topic.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	t := client.Topic(topic.Status.ProjectID)
	exists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return err
	}

	if !exists {
		if topic.Spec.PropagationPolicy == v1alpha1.TopicPolicyNoCreateNoDelete {
			logging.FromContext(ctx).Desugar().Error("Topic does not exist and the topic policy doesn't allow creation")
			return fmt.Errorf("Topic %q does not exist and the topic policy doesn't allow creation", topic.Spec.Topic)
		} else {
			// Create a new topic with the given name.
			t, err = client.CreateTopic(ctx, topic.Spec.Topic)
			if err != nil {
				// For some reason (maybe some cache invalidation thing), sometimes t.Exists returns that the topic
				// doesn't exist but it actually does. When we try to create it again, it fails with an AlreadyExists
				// reason. We check for that error here. If it happens, then return nil.
				if st, ok := gstatus.FromError(err); !ok {
					logging.FromContext(ctx).Desugar().Error("Failed from Pub/Sub client while creating topic", zap.Error(err))
					return err
				} else if st.Code() != codes.AlreadyExists {
					logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub topic", zap.Error(err))
					return err
				}
				return nil
			}
		}
	}
	return nil
}

// deleteTopic looks at the status.TopicID and if non-empty,
// hence indicating that we have created a topic successfully,
// remove it.
func (r *Reconciler) deleteTopic(ctx context.Context, topic *v1alpha1.Topic) error {
	if topic.Status.TopicID == "" {
		return nil
	}

	// At this point the project ID should have been populated in the status.
	// Querying Pub/Sub as the topic could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.createClientFn(ctx, topic.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	t := client.Topic(topic.Status.TopicID)
	exists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return err
	}
	if exists {
		// Delete the topic.
		if err := t.Delete(ctx); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to delete Pub/Sub topic", zap.Error(err))
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, original *v1alpha1.Topic, desired *v1alpha1.Topic) error {
	existing := original.DeepCopy()
	return reconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		// The first iteration tries to use the informer's state, subsequent attempts fetch the latest state via API.
		if attempts > 0 {
			existing, err = r.RunClientSet.PubsubV1alpha1().Topics(desired.Namespace).Get(desired.Name, metav1.GetOptions{})
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

		_, err = r.RunClientSet.PubsubV1alpha1().Topics(desired.Namespace).UpdateStatus(existing)
		if err == nil && becomesReady {
			// TODO compute duration since last non-ready. See https://github.com/google/knative-gcp/issues/455.
			duration := time.Since(existing.ObjectMeta.CreationTimestamp.Time)
			logging.FromContext(ctx).Desugar().Info("Topic became ready", zap.Any("after", duration))
			r.Recorder.Event(existing, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("Topic %q became ready", existing.Name))
			if metricErr := r.StatsReporter.ReportReady("Topic", existing.Namespace, existing.Name, duration); metricErr != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to record ready for Topic", zap.Error(metricErr))
			}
		}

		return err
	})
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (r *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.Topic) (*v1alpha1.Topic, bool, error) {
	source, err := r.topicLister.Topics(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := source.DeepCopy()

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

	update, err := r.RunClientSet.PubsubV1alpha1().Topics(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}

func addFinalizer(s *v1alpha1.Topic) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func removeFinalizer(s *v1alpha1.Topic) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) reconcilePublisher(ctx context.Context, topic *v1alpha1.Topic) (error, *servingv1.Service) {
	name := resources.GeneratePublisherName(topic)
	existing, err := r.serviceLister.Services(topic.Namespace).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Unable to get an existing publisher", zap.Error(err))
			return err, nil
		}
		existing = nil
	} else if !metav1.IsControlledBy(existing, topic) {
		p, _ := json.Marshal(existing)
		logging.FromContext(ctx).Desugar().Error("Topic does not own publisher service", zap.Any("publisher", p))
		return fmt.Errorf("Topic %q does not own publisher service: %q", topic.Name, name), nil
	}

	tracingCfg, err := tracing.ConfigToJSON(r.tracingConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing tracing config", zap.Error(err))
	}

	desired := resources.MakePublisher(&resources.PublisherArgs{
		Image:         r.publisherImage,
		Topic:         topic,
		Labels:        resources.GetLabels(controllerAgentName, topic.Name),
		TracingConfig: tracingCfg,
	})

	svc := existing
	if existing == nil {
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Create(desired)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create publisher", zap.Error(err))
			return err, nil
		}
	} else if !equality.Semantic.DeepEqual(&existing.Spec, &desired.Spec) {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Update(existing)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update publisher", zap.Any("publisher", existing), zap.Error(err))
			return err, nil
		}
	}
	return nil, svc
}

func (r *Reconciler) getPublisher(ctx context.Context, topic *v1alpha1.Topic) (*serving.Service, error) {
	pl, err := r.serviceLister.Services(topic.Namespace).List(resources.GetLabelSelector(controllerAgentName, topic.Name))

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list services: %v", zap.Error(err))
		return nil, err
	}
	for _, pub := range pl {
		if metav1.IsControlledBy(pub, topic) {
			return pub, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) UpdateFromTracingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		r.Logger.Error("Tracing ConfigMap is nil")
		return
	}
	delete(cfg.Data, "_example")

	tracingCfg, err := tracingconfig.NewTracingConfigFromConfigMap(cfg)
	if err != nil {
		r.Logger.Warnw("failed to create tracing config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.tracingConfig = tracingCfg
	r.Logger.Debugw("Updated Tracing config", zap.Any("tracingCfg", r.tracingConfig))
	// TODO: requeue all Topics. See https://github.com/google/knative-gcp/issues/457.
}
