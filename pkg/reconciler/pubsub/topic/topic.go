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

	"cloud.google.com/go/compute/metadata"
	googlepubsub "cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/tracing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	tracingconfig "knative.dev/pkg/tracing/config"
	serving "knative.dev/serving/pkg/apis/serving/v1"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/events/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/topic/resources"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Topics"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for Topic resources.
type Reconciler struct {
	*pubsub.PubSubBase

	// listers index properties about resources
	topicLister   listers.TopicLister
	serviceLister servinglisters.ServiceLister

	publisherImage string
	tracingConfig  *tracingconfig.Config
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
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	logger := logging.FromContext(ctx)

	// Get the Topic resource with this namespace/name
	original, err := r.topicLister.Topics(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("Topic %q in work queue no longer exists", key)
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
		logger.Warnw("Failed to update Topic finalizers", zap.Error(fErr))
		r.Recorder.Eventf(topic, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for Topic %q: %v", topic.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		r.Recorder.Eventf(topic, corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topic.GetName())
	}

	if equality.Semantic.DeepEqual(original.Status, topic.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, topic); uErr != nil {
		logger.Warnw("Failed to update Topic status", zap.Error(uErr))
		r.Recorder.Eventf(topic, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Topic %q: %v", topic.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(topic, corev1.EventTypeNormal, "Updated", "Updated Topic %q", topic.GetName())
	}
	if reconcileErr != nil {
		r.Recorder.Event(topic, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, topic *v1alpha1.Topic) error {
	logger := logging.FromContext(ctx)

	topic.Status.InitializeConditions()

	if topic.GetDeletionTimestamp() != nil {
		logger.Debug("Deleting topic", zap.Any("propagationPolicy", topic.Spec.PropagationPolicy))
		if topic.Spec.PropagationPolicy == v1alpha1.TopicPolicyCreateDelete {
			if err := r.deleteTopic(ctx, topic); err != nil {
				logger.Error("failed to delete topic", zap.Error(err))
				return err
			}
			topic.Status.MarkNoTopic("Deleted", "Successfully deleted topic %q.", topic.Status.TopicID)
			topic.Status.TopicID = ""
		}
		removeFinalizer(topic)
		return nil
	}

	// Set the topic being used.
	topic.Status.TopicID = topic.Spec.Topic

	// Add the finalizer.
	addFinalizer(topic)

	if err := r.reconcileTopic(ctx, topic); err != nil {
		topic.Status.MarkNoTopic("ReconcileFailed",
			"Failed to reconcile Topic: %q", err.Error())
		return err
	}
	topic.Status.MarkTopicReady()

	if err := r.createOrUpdatePublisher(ctx, topic); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) reconcileTopic(ctx context.Context, topic *v1alpha1.Topic) error {
	logger := logging.FromContext(ctx).With(zap.String("topic", topic.Spec.Topic))
	if topic.Spec.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			logger.Error("failed to find project id", zap.Error(err))
			return err
		}
		topic.Spec.Project = project
	}
	logger = logger.With(zap.String("project", topic.Spec.Project))

	client, err := googlepubsub.NewClient(ctx, topic.Spec.Project)
	if err != nil {
		logger.Error("failed to create Pub/Sub client", zap.Error(err))
		return err
	}

	t := client.Topic(topic.Spec.Topic)
	exists, err := t.Exists(ctx)
	if err != nil {
		logger.Error("failed to verify topic exists", zap.Error(err))
		return err
	}

	if !exists {
		if topic.Spec.PropagationPolicy == v1alpha1.TopicPolicyNoCreateNoDelete {
			logger.Error("topic does not exist", zap.Any("propagationPolicy", topic.Spec.PropagationPolicy))
			return fmt.Errorf("topic %q does not exist", topic.Spec.Topic)
		} else {
			// Create a new topic with the given name.
			t, err = client.CreateTopic(ctx, topic.Spec.Topic)
			if err != nil {
				logger.Error("failed to create topic", zap.Error(err))
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) deleteTopic(ctx context.Context, topic *v1alpha1.Topic) error {
	// At this point the project should have been populated.
	// Querying Pub/Sub as the topic could have been deleted outside the cluster (e.g, through gcloud).
	client, err := googlepubsub.NewClient(ctx, topic.Spec.Project)
	if err != nil {
		return err
	}
	t := client.Topic(topic.Spec.Topic)
	exists, err := t.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		// Delete the topic.
		if err := t.Delete(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Topic) (*v1alpha1.Topic, error) {
	topic, err := r.topicLister.Topics(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if equality.Semantic.DeepEqual(topic.Status, desired.Status) {
		return topic, nil
	}
	becomesReady := desired.Status.IsReady() && !topic.Status.IsReady()
	// Don't modify the informers copy.
	existing := topic.DeepCopy()
	existing.Status = desired.Status

	ch, err := r.RunClientSet.PubsubV1alpha1().Topics(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(ch.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Infof("Topic %q became ready after %v", topic.Name, duration)

		if err := r.StatsReporter.ReportReady("Topic", topic.Namespace, topic.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Topic, %v", err)
		}
	}

	return ch, err
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

func (r *Reconciler) createOrUpdatePublisher(ctx context.Context, topic *v1alpha1.Topic) error {
	name := resources.GeneratePublisherName(topic)
	existing, err := r.serviceLister.Services(topic.Namespace).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Error("Unable to get an existing publisher", zap.Error(err))
			return err
		}
		existing = nil
	} else if !metav1.IsControlledBy(existing, topic) {
		p, _ := json.Marshal(existing)
		logging.FromContext(ctx).Error("Got a pre-owned publisher", zap.Any("publisher", p))
		return fmt.Errorf("topic %q does not own service: %q", topic.Name, name)
	}

	tracingCfg, err := tracing.ConfigToJSON(r.tracingConfig)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error serializing tracing config", zap.Error(err))
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
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher created", zap.Any("publisher", svc))
	} else if !equality.Semantic.DeepEqual(&existing.Spec, &desired.Spec) {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Update(existing)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher updated", zap.Any("publisher", svc))
	} else {
		logging.FromContext(ctx).Desugar().Info("Reusing existing publisher", zap.Any("publisher", existing))
	}

	// Update the topic.
	topic.Status.PropagatePublisherStatus(svc.Status.GetCondition(apis.ConditionReady))
	if svc.Status.IsReady() {
		topic.Status.SetAddress(svc.Status.Address.URL)
	}
	return nil
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
	r.Logger.Infow("Updated Tracing config", zap.Any("tracingCfg", r.tracingConfig))
	// TODO: requeue all Topics.
}
