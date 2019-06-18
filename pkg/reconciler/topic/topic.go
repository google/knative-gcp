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
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	listers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/Topic/resources"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Topics"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for Topic resources.
type Reconciler struct {
	*pubsub.PubSubBase

	deploymentLister appsv1listers.DeploymentLister

	// listers index properties about resources
	topicLister listers.TopicLister

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

	// 1. create a topic.
	// 2. create all subscriptions that are in spec and not in status.
	// 3. delete all subscriptions that are in status but not in spec.
	// 4. deploy invoker with updated subscriptions to target list.
	// ?- how to update invoker with that list.
	// Bad options:
	// topic could make a topic resource and n PullSubscriptions per subscriber and that is it.
	//   downside is that there would be 1+n resources per topic and n subscribers.

	if topic.GetDeletionTimestamp() != nil {
		logger.Info("topic Deleting.")

		state, err := c.EnsureTopicDeleted(ctx, topic, topic.Spec.Project, topic.Status.TopicID)
		switch state {
		case pubsub.OpsGetFailedState:
			logger.Error("Failed to get Topic ops job.", zap.Any("state", state), zap.Error(err))
			return err

		case pubsub.OpsCreatedState:
			// If we created a job to make a subscription, then add the finalizer and update the status.
			topic.Status.MarkTopicOperating(
				"Deleting",
				"Created Job to delete topic %q.",
				topic.Status.TopicID)
			return nil

		case pubsub.OpsCompeteSuccessfulState:
			topic.Status.MarkNoTopic("Deleted", "Successfully deleted topic %q.", topic.Status.TopicID)
			topic.Status.TopicID = ""
			removeFinalizer(topic)

		case pubsub.OpsCreateFailedState, pubsub.OpsCompeteFailedState:
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
		return nil
	}

	topic.Status.TopicID = topic.Spec.Topic
	state, err := c.EnsureTopic(ctx, topic, topic.Spec.Project, topic.Status.TopicID)
	switch state {
	case pubsub.OpsGetFailedState:
		logger.Error("Failed to get topic ops job.", zap.Any("state", state), zap.Error(err))
		return err

	case pubsub.OpsCreatedState:
		// If we created a job to make a subscription, then add the finalizer and update the status.
		addFinalizer(topic)
		topic.Status.MarkTopicOperating("Creating",
			"Created Job to create topic %q.",
			topic.Status.TopicID)
		return nil

	case pubsub.OpsCompeteSuccessfulState:
		topic.Status.MarkTopicReady()

	case pubsub.OpsCreateFailedState, pubsub.OpsCompeteFailedState:
		logger.Error("Failed to create topic.", zap.Any("state", state), zap.Error(err))

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

	_, err = c.createPublisher(ctx, topic)
	if err != nil {
		logger.Error("Unable to create the invoker", zap.Error(err))
		return err
	}
	topic.Status.MarkDeployed()

	return nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Topic) (*v1alpha1.Topic, error) {
	topic, err := c.topicLister.Topics(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(topic.Status, desired.Status) {
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

func (c *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.Topic) (*v1alpha1.Topic, bool, error) {
	source, err := c.topicLister.Topics(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := source.DeepCopy()

	var finalizers []string

	// If there's nothing to update, just return.
	exisitingFinalizers := sets.NewString(existing.Finalizers...)
	desiredFinalizers := sets.NewString(desired.Finalizers...)

	if desiredFinalizers.Has(finalizerName) {
		if exisitingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Add the finalizer.
		finalizers = append(existing.Finalizers, finalizerName)
	} else {
		if !exisitingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Remove the finalizer.
		exisitingFinalizers.Delete(finalizerName)
		finalizers = exisitingFinalizers.List()
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

func (r *Reconciler) createPublisher(ctx context.Context, Topic *v1alpha1.Topic) (*appsv1.Deployment, error) {
	pub, err := r.getPublisher(ctx, Topic)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing publisher", zap.Error(err))
		return nil, err
	}
	if pub != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing publisher", zap.Any("publisher", pub))
		return pub, nil
	}
	dp := resources.MakePublisher(&resources.PublisherArgs{
		Image:  r.publisherImage,
		Topic:  Topic,
		Labels: resources.GetLabels(controllerAgentName, Topic.Name),
	})
	dp, err = r.KubeClientSet.AppsV1().Deployments(Topic.Namespace).Create(dp)
	logging.FromContext(ctx).Desugar().Info("Publisher created.", zap.Error(err), zap.Any("publisher", dp))
	return dp, err
}

func (r *Reconciler) getPublisher(ctx context.Context, topic *v1alpha1.Topic) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(topic.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(controllerAgentName, topic.Name).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, topic) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}
