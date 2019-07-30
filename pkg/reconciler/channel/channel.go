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
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/zap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	listers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/messaging/v1alpha1"
	pubsublisters "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/channel/resources"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Channels"
)

// Reconciler implements controller.Reconciler for Channel resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	channelLister      listers.ChannelLister
	topicLister        pubsublisters.TopicLister
	subscriptionLister pubsublisters.PullSubscriptionLister

	tracker tracker.Interface // TODO: use tracker for sink.
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
		logger.Errorf("Channel %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	if original.GetDeletionTimestamp() != nil {
		return nil
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

func (r *Reconciler) reconcile(ctx context.Context, channel *v1alpha1.Channel) error {
	channel.Status.InitializeConditions()

	if channel.Status.TopicID == "" {
		channel.Status.TopicID = resources.GenerateTopicName(channel.UID)
	}

	// 1. Create the Topic.
	if topic, err := r.createTopic(ctx, channel); err != nil {
		channel.Status.MarkNoTopic("CreateTopicFailed", "Error when attempting to create Topic.")
		return err
	} else {
		// Propagate Status.
		if c := topic.Status.GetCondition(pubsubv1alpha1.TopicConditionReady); c != nil {
			if c.IsTrue() {
				channel.Status.MarkTopicReady()
			} else if c.IsUnknown() {
				channel.Status.MarkTopicOperating(c.Reason, c.Message)
			} else if c.IsFalse() {
				channel.Status.MarkNoTopic(c.Reason, c.Message)
			}
		}
	}
	// 2. Sync all subscriptions.
	//   a. create all subscriptions that are in spec and not in status.
	//   b. delete all subscriptions that are in status but not in spec.
	if err := r.syncSubscribers(ctx, channel); err != nil {
		return err
	}

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

	ch, err := c.RunClientSet.MessagingV1alpha1().Channels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(ch.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("Channel %q became ready after %v", channel.Name, duration)

		if err := c.StatsReporter.ReportReady("Channel", channel.Namespace, channel.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Channel, %v", err)
		}
	}

	return ch, err
}

func (c *Reconciler) syncSubscribers(ctx context.Context, channel *v1alpha1.Channel) error {
	if channel.Status.Subscribers == nil {
		channel.Status.Subscribers = []eventingduck.SubscriberStatus(nil)
	}

	subCreates := []eventingduck.SubscriberSpec(nil)
	subUpdates := []eventingduck.SubscriberSpec(nil)
	subDeletes := []eventingduck.SubscriberStatus(nil)

	// Make a map of name to PullSubscription for lookup.
	pullsubs := make(map[string]pubsubv1alpha1.PullSubscription)
	if subs, err := c.getPullSubscriptions(ctx, channel); err != nil {
		c.Logger.Infof("Failed to list PullSubscriptions, %s", err)
	} else {
		for _, s := range subs {
			pullsubs[s.Name] = s
		}
	}

	exists := make(map[types.UID]eventingduck.SubscriberStatus)
	for _, s := range channel.Status.Subscribers {
		exists[s.UID] = s
	}

	if channel.Spec.Subscribable != nil {
		for _, want := range channel.Spec.Subscribable.Subscribers {
			if got, ok := exists[want.UID]; !ok {
				// If it does not exist, then update it.
				subCreates = append(subCreates, want)
			} else {
				_, found := pullsubs[resources.GenerateSubscriptionName(want.UID)]
				// If did not find or the PS has updated generation, update it.
				if !found || got.ObservedGeneration != want.Generation {
					subUpdates = append(subUpdates, want)
				}
			}
			// Remove want from exists.
			delete(exists, want.UID)
		}
	}

	// Remaining exists will be deleted.
	for _, e := range exists {
		subDeletes = append(subDeletes, e)
	}

	for _, s := range subCreates {
		genName := resources.GenerateSubscriptionName(s.UID)
		c.Logger.Infof("Channel %q will create subscription %s", channel.Name, genName)

		ps := resources.MakePullSubscription(&resources.PullSubscriptionArgs{
			Owner:      channel,
			Name:       genName,
			Project:    channel.Spec.Project,
			Topic:      channel.Status.TopicID,
			Secret:     channel.Spec.Secret,
			Labels:     resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, genName),
			Subscriber: s,
		})
		ps, err := c.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Create(ps)
		if err != nil {
			c.Recorder.Eventf(channel, corev1.EventTypeWarning, "CreateSubscriberFailed", "Creating Subscriber %q failed", genName)
			return err
		}
		c.Recorder.Eventf(channel, corev1.EventTypeNormal, "CreatedSubscriber", "Created Subscriber %q", genName)

		channel.Status.Subscribers = append(channel.Status.Subscribers, eventingduck.SubscriberStatus{
			UID:                s.UID,
			ObservedGeneration: s.Generation,
			// TODO: do I need the other fields?
		})
		return nil // Signal a re-reconcile.
	}
	for _, s := range subUpdates {
		genName := resources.GenerateSubscriptionName(s.UID)
		c.Logger.Infof("Channel %q will update subscription %s", channel.Name, genName)

		ps := resources.MakePullSubscription(&resources.PullSubscriptionArgs{
			Owner:      channel,
			Name:       genName,
			Project:    channel.Spec.Project,
			Topic:      channel.Status.TopicID,
			Secret:     channel.Spec.Secret,
			Labels:     resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, genName),
			Subscriber: s,
		})

		existingPs, found := pullsubs[genName]
		if !found {
			// PullSubscription does not exist, that's ok, create it now.
			ps, err := c.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Create(ps)
			if err != nil {
				c.Recorder.Eventf(channel, corev1.EventTypeWarning, "CreateSubscriberFailed", "Creating Subscriber %q failed", genName)
				return err
			}
			c.Recorder.Eventf(channel, corev1.EventTypeNormal, "CreatedSubscriber", "Created Subscriber %q", ps.Name)
		} else if !equality.Semantic.DeepEqual(ps.Spec, existingPs.Spec) {
			ps, err := c.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Update(ps)
			if err != nil {
				c.Recorder.Eventf(channel, corev1.EventTypeWarning, "UpdateSubscriberFailed", "Updating Subscriber %q failed", genName)
				return err
			}
			c.Recorder.Eventf(channel, corev1.EventTypeNormal, "UpdatedSubscriber", "Updated Subscriber %q", ps.Name)
		}
		for i, ss := range channel.Status.Subscribers {
			if ss.UID == s.UID {
				channel.Status.Subscribers[i].ObservedGeneration = s.Generation
			}
		}
		return nil
	}
	for _, s := range subDeletes {
		genName := resources.GenerateSubscriptionName(s.UID)
		c.Logger.Infof("Channel %q will delete subscription %s", channel.Name, genName)
		// TODO: we need to handle the case of a already deleted pull subscription. Perhaps move to ensure deleted method.
		if err := c.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Delete(genName, &metav1.DeleteOptions{}); err != nil {
			c.Logger.Errorf("unable to delete PullSubscription %s for Channel %q, %s", genName, channel.Name, err.Error())
			c.Recorder.Eventf(channel, corev1.EventTypeWarning, "DeleteSubscriberFailed", "Deleting Subscriber %q failed", genName)
			return err
		}
		c.Recorder.Eventf(channel, corev1.EventTypeNormal, "DeletedSubscriber", "Deleted Subscriber %q", genName)

		for i, ss := range channel.Status.Subscribers {
			if ss.UID == s.UID {
				// Swap len-1 with i and then pop len-1 off the slice.
				channel.Status.Subscribers[i] = channel.Status.Subscribers[len(channel.Status.Subscribers)-1]
				channel.Status.Subscribers = channel.Status.Subscribers[:len(channel.Status.Subscribers)-1]
			}
		}
		return nil // Signal a re-reconcile.
	}

	return nil
}

func (r *Reconciler) createTopic(ctx context.Context, channel *v1alpha1.Channel) (*pubsubv1alpha1.Topic, error) {
	topic, err := r.getTopic(ctx, channel)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an Topic", zap.Error(err))
		return nil, err
	}
	if topic != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing Topic", zap.Any("topic", topic))
		if topic.Status.Address != nil {
			channel.Status.SetAddress(topic.Status.Address.URL)
		} else {
			channel.Status.SetAddress(nil)
		}
		return topic, nil
	}
	topic, err = r.RunClientSet.PubsubV1alpha1().Topics(channel.Namespace).Create(resources.MakeTopic(&resources.TopicArgs{
		Owner:   channel,
		Name:    resources.GenerateTopicName(channel.UID),
		Project: channel.Spec.Project,
		Secret:  channel.Spec.Secret,
		Topic:   channel.Status.TopicID,
		Labels:  resources.GetLabels(controllerAgentName, channel.Name),
	}))
	if err != nil {
		logging.FromContext(ctx).Info("Topic created.", zap.Error(err), zap.Any("topic", topic))
		r.Recorder.Eventf(channel, corev1.EventTypeNormal, "TopicCreated", "Created Topic %q", topic.GetName())
	}
	return topic, err
}

func (r *Reconciler) getTopic(ctx context.Context, channel *v1alpha1.Channel) (*pubsubv1alpha1.Topic, error) {
	tl, err := r.RunClientSet.PubsubV1alpha1().Topics(channel.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(controllerAgentName, channel.Name).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "Channel",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Error("Unable to list topics: %v", zap.Error(err))
		return nil, err
	}
	for _, topic := range tl.Items {
		if metav1.IsControlledBy(&topic, channel) {
			return &topic, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) getPullSubscriptions(ctx context.Context, channel *v1alpha1.Channel) ([]pubsubv1alpha1.PullSubscription, error) {
	sl, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).List(metav1.ListOptions{
		// Use GetLabelSelector to select all PullSubscriptions related to this channel.
		LabelSelector: resources.GetLabelSelector(controllerAgentName, channel.Name).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "Channel",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Error("Unable to list pullsubscriptions: %v", zap.Error(err))
		return nil, err
	}
	subs := []pubsubv1alpha1.PullSubscription(nil)
	for _, subscription := range sl.Items {
		if metav1.IsControlledBy(&subscription, channel) {
			subs = append(subs, subscription)
		}
	}
	return subs, nil
}
