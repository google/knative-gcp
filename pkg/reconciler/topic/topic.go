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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"
	servingv1alpha1listers "knative.dev/serving/pkg/client/listers/serving/v1alpha1"
	servingv1beta1listers "knative.dev/serving/pkg/client/listers/serving/v1beta1"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	listers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/topic/resources"
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
	topicLister listers.TopicLister

	servingVersion        string
	serviceV1alpha1Lister servingv1alpha1listers.ServiceLister
	serviceV1beta1Lister  servingv1beta1listers.ServiceLister

	publisherImage string
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Service resource
// with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	logger := logging.FromContext(ctx)

	// Get the Topic resource with this namespace/name
	original, err := c.topicLister.Topics(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("service %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	Topic := original.DeepCopy()

	// Reconcile this copy of the Topic and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = c.reconcile(ctx, Topic)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		Topic.Status.ObservedGeneration = Topic.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, Topic.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := c.updateFinalizers(ctx, Topic); fErr != nil {
		logger.Warnw("Failed to update Topic finalizers", zap.Error(fErr))
		c.Recorder.Eventf(Topic, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for Topic %q: %v", Topic.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		c.Recorder.Eventf(Topic, corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", Topic.GetName())
	}

	if equality.Semantic.DeepEqual(original.Status, Topic.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := c.updateStatus(ctx, Topic); uErr != nil {
		logger.Warnw("Failed to update Topic status", zap.Error(uErr))
		c.Recorder.Eventf(Topic, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Topic %q: %v", Topic.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		c.Recorder.Eventf(Topic, corev1.EventTypeNormal, "Updated", "Updated Topic %q", Topic.GetName())
	}
	if reconcileErr != nil {
		c.Recorder.Event(Topic, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, topic *v1alpha1.Topic) error {
	logger := logging.FromContext(ctx)

	topic.Status.InitializeConditions()

	if topic.GetDeletionTimestamp() != nil {
		logger.Debug("Topic Deleting.", zap.Any("propagationPolicy", topic.Spec.PropagationPolicy))

		if topic.Spec.PropagationPolicy == v1alpha1.TopicPolicyCreateDelete {
			// Ensure the Topic is deleted.
			state, err := c.EnsureTopicDeleted(ctx, topic, *topic.Spec.Secret, topic.Spec.Project, topic.Status.TopicID)
			switch state {
			case pubsub.OpsJobGetFailed:
				logger.Error("Failed to get Topic ops job.", zap.Any("state", state), zap.Error(err))
				return err

			case pubsub.OpsJobCreated:
				// If we created a job to delete a topic, update the status.
				topic.Status.MarkTopicOperating(
					"Deleting",
					"Created Job to delete topic %q.",
					topic.Status.TopicID)
				return nil

			case pubsub.OpsJobCompleteSuccessful:
				topic.Status.MarkNoTopic("Deleted", "Successfully deleted topic %q.", topic.Status.TopicID)
				topic.Status.TopicID = ""
				removeFinalizer(topic)

			case pubsub.OpsJobCreateFailed, pubsub.OpsJobCompleteFailed:
				logger.Error("Failed to delete topic.", zap.Any("state", state), zap.Error(err))

				msg := "unknown"
				if err != nil {
					msg = err.Error()
				}
				topic.Status.MarkNoTopic(
					"DeleteFailed",
					"Failed to delete topic: %q.",
					msg)
				return err
			}
		} else {
			removeFinalizer(topic)
		}

		return nil
	}

	// Set the topic being used.
	topic.Status.TopicID = topic.Spec.Topic

	switch topic.Spec.PropagationPolicy {
	case v1alpha1.TopicPolicyCreateDelete, v1alpha1.TopicPolicyCreateNoDelete:
		state, err := c.EnsureTopicCreated(ctx, topic, *topic.Spec.Secret, topic.Spec.Project, topic.Status.TopicID)
		// Check state.
		switch state {
		case pubsub.OpsJobGetFailed:
			logger.Error("Failed to get topic ops job.",
				zap.Any("propagationPolicy", topic.Spec.PropagationPolicy),
				zap.Any("state", state),
				zap.Error(err))
			return err

		case pubsub.OpsJobCreated:
			// If we created a job to make a topic, then add the finalizer and update the status.
			addFinalizer(topic)
			topic.Status.MarkTopicOperating("Creating",
				"Created Job to create topic %q.",
				topic.Status.TopicID)
			return nil

		case pubsub.OpsJobCompleteSuccessful:
			topic.Status.MarkTopicReady()

		case pubsub.OpsJobCreateFailed, pubsub.OpsJobCompleteFailed:
			logger.Error("Failed to create topic.",
				zap.Any("propagationPolicy", topic.Spec.PropagationPolicy),
				zap.Any("state", state),
				zap.Error(err))

			msg := "unknown"
			if err != nil {
				msg = err.Error()
			}
			topic.Status.MarkNoTopic(
				"CreateFailed",
				"Failed to create Topic: %q",
				msg)
			return err
		}

	case v1alpha1.TopicPolicyNoCreateNoDelete:
		state, err := c.EnsureTopicExists(ctx, topic, *topic.Spec.Secret, topic.Spec.Project, topic.Status.TopicID)
		// Check state.
		switch state {
		case pubsub.OpsJobGetFailed:
			logger.Error("Failed to get topic ops job.",
				zap.Any("propagationPolicy", topic.Spec.PropagationPolicy),
				zap.Any("state", state),
				zap.Error(err))
			return err

		case pubsub.OpsJobCreated:
			// If we created a job to verify a topic, then update the status.
			topic.Status.MarkTopicOperating("Verifying",
				"Created Job to verify topic %q.",
				topic.Status.TopicID)
			return nil

		case pubsub.OpsJobCompleteSuccessful:
			topic.Status.MarkTopicReady()

		case pubsub.OpsJobCreateFailed, pubsub.OpsJobCompleteFailed:
			logger.Error("Failed to verify topic.",
				zap.Any("propagationPolicy", topic.Spec.PropagationPolicy),
				zap.Any("state", state),
				zap.Error(err))

			msg := "unknown"
			if err != nil {
				msg = err.Error()
			}
			topic.Status.MarkNoTopic(
				"VerifyFailed",
				"Failed to verify Topic: %q",
				msg)
			return err
		}
	default:
		logger.Error("Unknown propagation policy.", zap.Any("propagationPolicy", topic.Spec.PropagationPolicy))
		return nil
	}

	switch c.servingVersion {
	case "", "v1alpha1":
		if err := c.createOrUpdatePublisherV1alpha1(ctx, topic); err != nil {
			logger.Error("Unable to create the publisher@v1alpha1", zap.Error(err))
			return err
		}
	case "v1beta1":
		if err := c.createOrUpdatePublisherV1beta1(ctx, topic); err != nil {
			logger.Error("Unable to create the publisher@v1beta1", zap.Error(err))
			return err
		}
	default:
		logger.Error("unknown serving version selected: %q", c.servingVersion)
	}

	return nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Topic) (*v1alpha1.Topic, error) {
	topic, err := c.topicLister.Topics(desired.Namespace).Get(desired.Name)
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

	ch, err := c.RunClientSet.PubsubV1alpha1().Topics(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(ch.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("Topic %q became ready after %v", topic.Name, duration)

		if err := c.StatsReporter.ReportReady("Topic", topic.Namespace, topic.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Topic, %v", err)
		}
	}

	return ch, err
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (c *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.Topic) (*v1alpha1.Topic, bool, error) {
	source, err := c.topicLister.Topics(desired.Namespace).Get(desired.Name)
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

	update, err := c.RunClientSet.PubsubV1alpha1().Topics(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
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

func (r *Reconciler) createOrUpdatePublisherV1alpha1(ctx context.Context, topic *v1alpha1.Topic) error {
	name := resources.GeneratePublisherName(topic)
	existing, err := r.ServingClientSet.ServingV1alpha1().Services(topic.Namespace).Get(name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing publisher", zap.Error(err))
		return err
	}
	if existing != nil && !metav1.IsControlledBy(existing, topic) {
		return fmt.Errorf("Topic: %s does not own Service: %s", topic.Name, name)
	}

	desired := resources.MakePublisherV1alpha1(&resources.PublisherArgs{
		Image:  r.publisherImage,
		Topic:  topic,
		Labels: resources.GetLabels(controllerAgentName, topic.Name),
	})

	svc := existing
	if existing == nil {
		svc, err = r.ServingClientSet.ServingV1alpha1().Services(topic.Namespace).Create(desired)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher created.", zap.Error(err), zap.Any("publisher", svc))
	} else if diff := cmp.Diff(desired.Spec, existing.Spec); diff != "" {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1alpha1().Services(topic.Namespace).Update(existing)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher updated.",
			zap.Error(err), zap.Any("publisher", svc), zap.String("diff", diff))
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

func (r *Reconciler) createOrUpdatePublisherV1beta1(ctx context.Context, topic *v1alpha1.Topic) error {
	name := resources.GeneratePublisherName(topic)
	existing, err := r.ServingClientSet.ServingV1beta1().Services(topic.Namespace).Get(name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing publisher", zap.Error(err))
		return err
	}
	if existing != nil && !metav1.IsControlledBy(existing, topic) {
		return fmt.Errorf("Topic: %s does not own Service: %s", topic.Name, name)
	}

	desired := resources.MakePublisherV1beta1(&resources.PublisherArgs{
		Image:  r.publisherImage,
		Topic:  topic,
		Labels: resources.GetLabels(controllerAgentName, topic.Name),
	})

	svc := existing
	if existing == nil {
		svc, err = r.ServingClientSet.ServingV1beta1().Services(topic.Namespace).Create(desired)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher created.", zap.Error(err), zap.Any("publisher", svc))
	} else if diff := cmp.Diff(desired.Spec, existing.Spec); diff != "" {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1beta1().Services(topic.Namespace).Update(existing)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher updated.",
			zap.Error(err), zap.Any("publisher", svc), zap.String("diff", diff))
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

func (r *Reconciler) getPublisher(ctx context.Context, topic *v1alpha1.Topic) (*servingv1beta1.Service, error) {
	pl, err := r.ServingClientSet.ServingV1beta1().Services(topic.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(controllerAgentName, topic.Name).String(),
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list services: %v", zap.Error(err))
		return nil, err
	}
	for _, pub := range pl.Items {
		if metav1.IsControlledBy(&pub, topic) {
			return &pub, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}
