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

package channel

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
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	listers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/channel/resources"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Channels"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for Channel resources.
type Reconciler struct {
	*pubsub.PubSubBase

	deploymentLister appsv1listers.DeploymentLister

	// listers index properties about resources
	channelLister listers.ChannelLister

	tracker tracker.Interface // TODO: use tracker for sink.

	invokerImage string

	//	eventTypeReconciler eventtype.Reconciler // TODO: event types.

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

	// Get the Channel resource with this namespace/name
	original, err := c.channelLister.Channels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("service %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	channel := original.DeepCopy()

	// Reconcile this copy of the channel and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = c.reconcile(ctx, channel)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		channel.Status.ObservedGeneration = channel.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, channel.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := c.updateFinalizers(ctx, channel); fErr != nil {
		logger.Warnw("Failed to update Channel finalizers", zap.Error(fErr))
		c.Recorder.Eventf(channel, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for Channel %q: %v", channel.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		c.Recorder.Eventf(channel, corev1.EventTypeNormal, "Updated", "Updated Channel %q finalizers", channel.GetName())
	}

	if equality.Semantic.DeepEqual(original.Status, channel.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := c.updateStatus(ctx, channel); uErr != nil {
		logger.Warnw("Failed to update Channel status", zap.Error(uErr))
		c.Recorder.Eventf(channel, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Channel %q: %v", channel.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		c.Recorder.Eventf(channel, corev1.EventTypeNormal, "Updated", "Updated Channel %q", channel.GetName())
	}
	if reconcileErr != nil {
		c.Recorder.Event(channel, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, channel *v1alpha1.Channel) error {
	logger := logging.FromContext(ctx)

	channel.Status.InitializeConditions()

	// 1. create a topic.
	// 2. create all subscriptions that are in spec and not in status.
	// 3. delete all subscriptions that are in status but not in spec.
	// 4. deploy invoker with updated subscriptions to target list.
	// ?- how to update invoker with that list.
	// Bad options:
	// channel could make a Topic resource and n PullSubscriptions per subscriber and that is it.
	//   downside is that there would be 1+n resources per channel and n subscribers.

	if channel.GetDeletionTimestamp() != nil {
		logger.Info("Channel Deleting.")

		state, err := c.EnsureTopicDeleted(ctx, channel, channel.Spec.Project, channel.Status.TopicID)
		switch state {
		case pubsub.OpsGetFailedState:
			logger.Error("Failed to get Topic ops job.", zap.Any("state", state), zap.Error(err))
			return err

		case pubsub.OpsCreatedState:
			// If we created a job to make a subscription, then add the finalizer and update the status.
			channel.Status.MarkTopicOperating(
				"Deleting",
				"Created Job to delete Topic %q.",
				channel.Status.TopicID)
			return nil

		case pubsub.OpsCompeteSuccessfulState:
			channel.Status.MarkNoTopic("Deleted", "Successfully deleted Topic %q.", channel.Status.TopicID)
			channel.Status.TopicID = ""
			removeFinalizer(channel)

		case pubsub.OpsCreateFailedState, pubsub.OpsCompeteFailedState:
			logger.Error("Failed to delete subscription.", zap.Any("state", state), zap.Error(err))

			msg := "unknown"
			if err != nil {
				msg = err.Error()
			}
			channel.Status.MarkNoTopic(
				"DeleteFailed",
				"Failed to delete Topic: %q.",
				msg)
			return err
		}
		return nil
	}

	// TODO: there will be a lot of these jobs, we should collect them and run them all at the same time.

	channel.Status.TopicID = resources.GenerateTopicName(channel)
	state, err := c.EnsureTopic(ctx, channel, channel.Spec.Project, channel.Status.TopicID)
	switch state {
	case pubsub.OpsGetFailedState:
		logger.Error("Failed to get topic ops job.", zap.Any("state", state), zap.Error(err))
		return err

	case pubsub.OpsCreatedState:
		// If we created a job to make a subscription, then add the finalizer and update the status.
		addFinalizer(channel)
		channel.Status.MarkTopicOperating("Creating",
			"Created Job to create Topic %q.",
			channel.Status.TopicID)
		return nil

	case pubsub.OpsCompeteSuccessfulState:
		channel.Status.MarkTopicReady()

	case pubsub.OpsCreateFailedState, pubsub.OpsCompeteFailedState:
		logger.Error("Failed to create Topic.", zap.Any("state", state), zap.Error(err))

		msg := "unknown"
		if err != nil {
			msg = err.Error()
		}
		channel.Status.MarkNoTopic(
			"CreateFailed",
			"Failed to create Topic: %q",
			msg)
		return err
	}

	_, err = c.createInvoker(ctx, channel)
	if err != nil {
		logger.Error("Unable to create the invoker", zap.Error(err))
		return err
	}
	channel.Status.MarkDeployed()

	return nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Channel) (*v1alpha1.Channel, error) {
	channel, err := c.channelLister.Channels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(channel.Status, desired.Status) {
		return channel, nil
	}
	becomesReady := desired.Status.IsReady() && !channel.Status.IsReady()
	// Don't modify the informers copy.
	existing := channel.DeepCopy()
	existing.Status = desired.Status

	ch, err := c.RunClientSet.PubsubV1alpha1().Channels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(ch.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("Channel %q became ready after %v", channel.Name, duration)

		if err := c.StatsReporter.ReportReady("Channel", channel.Namespace, channel.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Channel, %v", err)
		}
	}

	return ch, err
}

func (c *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.Channel) (*v1alpha1.Channel, bool, error) {
	source, err := c.channelLister.Channels(desired.Namespace).Get(desired.Name)
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

	update, err := c.RunClientSet.PubsubV1alpha1().Channels(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}

func addFinalizer(s *v1alpha1.Channel) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func removeFinalizer(s *v1alpha1.Channel) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) createInvoker(ctx context.Context, channel *v1alpha1.Channel) (*appsv1.Deployment, error) {
	ra, err := r.getInvoker(ctx, channel)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing invoker", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing invoker", zap.Any("invoker", ra))
		return ra, nil
	}
	dp := resources.MakeInvoker(&resources.InvokerArgs{
		Image:   r.invokerImage,
		Channel: channel,
		Labels:  resources.GetLabels(controllerAgentName, channel.Name),
	})
	dp, err = r.KubeClientSet.AppsV1().Deployments(channel.Namespace).Create(dp)
	logging.FromContext(ctx).Desugar().Info("Invoker created.", zap.Error(err), zap.Any("invoker", dp))
	return dp, err
}

func (r *Reconciler) getInvoker(ctx context.Context, src *v1alpha1.Channel) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(controllerAgentName, src.Name).String(),
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
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}
