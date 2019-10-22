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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	serving "knative.dev/serving/pkg/apis/serving/v1"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	ops "github.com/google/knative-gcp/pkg/operations"
	pubsubOps "github.com/google/knative-gcp/pkg/operations/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/topic/resources"
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

	serviceLister servinglisters.ServiceLister

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
			case ops.OpsJobGetFailed:
				logger.Error("Failed to get Topic ops job.", zap.Any("state", state), zap.Error(err))
				return err

			case ops.OpsJobCreated:
				// If we created a job to delete a topic, update the status.
				topic.Status.MarkTopicOperating(
					"Deleting",
					"Created Job to delete topic %q.",
					topic.Status.TopicID)
				return nil

			case ops.OpsJobCompleteSuccessful:
				topic.Status.MarkNoTopic("Deleted", "Successfully deleted topic %q.", topic.Status.TopicID)
				topic.Status.TopicID = ""
				removeFinalizer(topic)
				return c.updateSecretFinailizer(ctx, topic, false)

			case ops.OpsJobCreateFailed, ops.OpsJobCompleteFailed:
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
			return c.updateSecretFinailizer(ctx, topic, false)
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
		case ops.OpsJobGetFailed:
			logger.Error("Failed to get topic ops job.",
				zap.Any("propagationPolicy", topic.Spec.PropagationPolicy),
				zap.Any("state", state),
				zap.Error(err))
			return err

		case ops.OpsJobCreated:
			// If we created a job to make a topic, then add the finalizer and update the status.
			addFinalizer(topic)
			topic.Status.MarkTopicOperating("Creating",
				"Created Job to create topic %q.",
				topic.Status.TopicID)
			return c.updateSecretFinailizer(ctx, topic, true)

		case ops.OpsJobCompleteSuccessful:
			topic.Status.MarkTopicReady()

			// Now that the job completed, grab the pod status for it.
			// Note that since it's not yet currently relied upon, don't hard
			// fail this if we can't fetch it. Just warn
			jobName := pubsubOps.TopicJobName(topic, "create")
			logger.Info("Finding job pods for", zap.String("jobName", jobName))

			var tar pubsubOps.TopicActionResult
			if err := c.UnmarshalJobTerminationMessage(ctx, topic.Namespace, jobName, &tar); err != nil {
				logger.Error("Failed to unmarshal termination message", zap.Error(err))
			} else {
				c.Logger.Infof("Topic project: %q", tar.ProjectId)
				topic.Status.ProjectID = tar.ProjectId
			}

		case ops.OpsJobCreateFailed, ops.OpsJobCompleteFailed:
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
		case ops.OpsJobGetFailed:
			logger.Error("Failed to get topic ops job.",
				zap.Any("propagationPolicy", topic.Spec.PropagationPolicy),
				zap.Any("state", state),
				zap.Error(err))
			return err

		case ops.OpsJobCreated:
			// If we created a job to verify a topic, then update the status.
			topic.Status.MarkTopicOperating("Verifying",
				"Created Job to verify topic %q.",
				topic.Status.TopicID)
			return nil

		case ops.OpsJobCompleteSuccessful:
			topic.Status.MarkTopicReady()

		case ops.OpsJobCreateFailed, ops.OpsJobCompleteFailed:
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

	if err := c.createOrUpdatePublisher(ctx, topic); err != nil {
		logger.Error("Unable to create the publisher", zap.Error(err))
		return err
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

// updateSecretFinailizer adds or deletes the finalizer on the secret used by the Topic.
func (c *Reconciler) updateSecretFinailizer(ctx context.Context, desired *v1alpha1.Topic, ensureFinalizer bool) error {
	tl, err := c.topicLister.Topics(desired.Namespace).List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to list Topics", zap.Error(err))
		return err
	}
	// Only delete the finalizer if this Topic is the last one
	// references the Secret.
	if !ensureFinalizer && !(len(tl) == 1 && tl[0].Name == desired.Name) {
		return nil
	}

	secret, err := c.KubeClientSet.CoreV1().Secrets(desired.Namespace).Get(desired.Spec.Secret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	existing := secret.DeepCopy()
	existingFinalizers := sets.NewString(existing.Finalizers...)
	hasFinalizer := existingFinalizers.Has(finalizerName)

	if ensureFinalizer == hasFinalizer {
		return nil
	}

	var desiredFinalizers []string
	if ensureFinalizer {
		desiredFinalizers = append(existing.Finalizers, finalizerName)
	} else {
		existingFinalizers.Delete(finalizerName)
		desiredFinalizers = existingFinalizers.List()
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      desiredFinalizers,
			"resourceVersion": existing.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = c.KubeClientSet.CoreV1().Secrets(existing.GetNamespace()).Patch(existing.GetName(), types.MergePatchType, patch)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to update Topic Secret's finalizers", zap.Error(err))
	}
	return err
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

func (r *Reconciler) createOrUpdatePublisher(ctx context.Context, topic *v1alpha1.Topic) error {
	name := resources.GeneratePublisherName(topic)
	existing, err := r.ServingClientSet.ServingV1().Services(topic.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Error("Unable to get an existing publisher", zap.Error(err))
			return err
		}
		existing = nil
	} else if !metav1.IsControlledBy(existing, topic) {
		p, _ := json.Marshal(existing)
		logging.FromContext(ctx).Error("Got a preowned publisher", zap.Any("publisher", string(p)))
		return fmt.Errorf("Topic: %s does not own Service: %s", topic.Name, name)
	}

	desired := resources.MakePublisher(&resources.PublisherArgs{
		Image:  r.publisherImage,
		Topic:  topic,
		Labels: resources.GetLabels(controllerAgentName, topic.Name),
	})

	svc := existing
	if existing == nil {
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Create(desired)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher created.", zap.Error(err), zap.Any("publisher", svc))
	} else if !equality.Semantic.DeepEqual(&existing.Spec, &desired.Spec) {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Update(existing)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Publisher updated.",
			zap.Error(err), zap.Any("publisher", svc))
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
	pl, err := r.ServingClientSet.ServingV1().Services(topic.Namespace).List(metav1.ListOptions{
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
