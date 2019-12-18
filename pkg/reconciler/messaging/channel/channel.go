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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/messaging/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/messaging/channel/resources"
)

// Reconciler implements controller.Reconciler for Channel resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	channelLister          listers.ChannelLister
	topicLister            pubsublisters.TopicLister
	pullSubscriptionLister pubsublisters.PullSubscriptionLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the Channel resource with this namespace/name
	original, err := r.channelLister.Channels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("Channel in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	channel := original.DeepCopy()

	// Reconcile this copy of the Channel and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = r.reconcile(ctx, channel)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		channel.Status.ObservedGeneration = channel.Generation
	}

	if equality.Semantic.DeepEqual(original.Status, channel.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, channel); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Channel status", zap.Error(uErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Channel %q: %v", channel.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(channel, corev1.EventTypeNormal, "Updated", "Updated Channel %q", channel.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(channel, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, channel *v1alpha1.Channel) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("channel", channel)))

	channel.Status.InitializeConditions()

	if channel.DeletionTimestamp != nil {
		return nil
	}

	// 1. Create the Topic.
	topic, err := r.createTopic(ctx, channel)
	if err != nil {
		channel.Status.MarkNoTopic("TopicCreateFailed", "Error when attempting to create Topic.")
		return err
	}
	channel.Status.PropagateTopicStatus(topic.Status.GetCondition(pubsubv1alpha1.TopicConditionReady))
	channel.Status.TopicID = topic.Spec.Topic

	// 2. Sync all subscriptions.
	//   a. create all subscriptions that are in spec and not in status.
	//   b. delete all subscriptions that are in status but not in spec.
	if err := r.syncSubscribers(ctx, channel); err != nil {
		return err
	}

	// 3. Sync all subscriptions statuses.
	if err := r.syncSubscribersStatus(ctx, channel); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Channel) (*v1alpha1.Channel, error) {
	channel, err := r.channelLister.Channels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if equality.Semantic.DeepEqual(channel.Status, desired.Status) {
		return channel, nil
	}
	becomesReady := desired.Status.IsReady() && !channel.Status.IsReady()
	// Don't modify the informers copy.
	existing := channel.DeepCopy()
	existing.Status = desired.Status

	ch, err := r.RunClientSet.MessagingV1alpha1().Channels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(ch.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Desugar().Info("Channel became ready", zap.Any("after", duration))

		if err := r.StatsReporter.ReportReady("Channel", channel.Namespace, channel.Name, duration); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to record ready for Channel", zap.Error(err))
		}
	}

	return ch, err
}

func (r *Reconciler) syncSubscribers(ctx context.Context, channel *v1alpha1.Channel) error {
	if channel.Status.SubscribableStatus == nil {
		channel.Status.SubscribableStatus = &eventingduck.SubscribableStatus{
			Subscribers: make([]eventingduck.SubscriberStatus, 0),
		}
	} else if channel.Status.SubscribableStatus.Subscribers == nil {
		channel.Status.SubscribableStatus.Subscribers = make([]eventingduck.SubscriberStatus, 0)
	}

	subCreates := []eventingduck.SubscriberSpec(nil)
	subUpdates := []eventingduck.SubscriberSpec(nil)
	subDeletes := []eventingduck.SubscriberStatus(nil)

	// Make a map of name to PullSubscription for lookup.
	pullsubs := make(map[string]pubsubv1alpha1.PullSubscription)
	if subs, err := r.getPullSubscriptions(ctx, channel); err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to list PullSubscriptions", zap.Error(err))
	} else {
		for _, s := range subs {
			pullsubs[s.Name] = s
		}
	}

	exists := make(map[types.UID]eventingduck.SubscriberStatus)
	for _, s := range channel.Status.SubscribableStatus.Subscribers {
		exists[s.UID] = s
	}

	if channel.Spec.Subscribable != nil {
		for _, want := range channel.Spec.Subscribable.Subscribers {
			if got, ok := exists[want.UID]; !ok {
				// If it does not exist, then create it.
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

		ps := resources.MakePullSubscription(&resources.PullSubscriptionArgs{
			Owner:       channel,
			Name:        genName,
			Project:     channel.Spec.Project,
			Topic:       channel.Status.TopicID,
			Secret:      channel.Spec.Secret,
			Labels:      resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, genName, string(channel.UID)),
			Annotations: resources.GetPullSubscriptionAnnotations(channel.Name),
			Subscriber:  s,
		})
		ps, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Create(ps)
		if apierrs.IsAlreadyExists(err) {
			// If the pullsub already exists and is owned by the current channel, mark it for update.
			if _, found := pullsubs[genName]; found {
				subUpdates = append(subUpdates, s)
			} else {
				r.Recorder.Eventf(channel, corev1.EventTypeWarning, "SubscriberNotOwned", "Subscriber %q is not owned by this channel", genName)
				return fmt.Errorf("channel %q does not own subscriber %q", channel.Name, genName)
			}
		} else if err != nil {
			r.Recorder.Eventf(channel, corev1.EventTypeWarning, "SubscriberCreateFailed", "Creating Subscriber %q failed", genName)
			return err
		}
		r.Recorder.Eventf(channel, corev1.EventTypeNormal, "SubscriberCreated", "Created Subscriber %q", genName)

		channel.Status.SubscribableStatus.Subscribers = append(channel.Status.SubscribableStatus.Subscribers, eventingduck.SubscriberStatus{
			UID:                s.UID,
			ObservedGeneration: s.Generation,
		})
		return nil // Signal a re-reconcile.
	}
	for _, s := range subUpdates {
		genName := resources.GenerateSubscriptionName(s.UID)

		ps := resources.MakePullSubscription(&resources.PullSubscriptionArgs{
			Owner:       channel,
			Name:        genName,
			Project:     channel.Spec.Project,
			Topic:       channel.Status.TopicID,
			Secret:      channel.Spec.Secret,
			Labels:      resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, genName, string(channel.UID)),
			Annotations: resources.GetPullSubscriptionAnnotations(channel.Name),
			Subscriber:  s,
		})

		existingPs, found := pullsubs[genName]
		if !found {
			// PullSubscription does not exist, that's ok, create it now.
			ps, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Create(ps)
			if apierrs.IsAlreadyExists(err) {
				// If the pullsub is not owned by the current channel, this is an error.
				r.Recorder.Eventf(channel, corev1.EventTypeWarning, "SubscriberNotOwned", "Subscriber %q is not owned by this channel", genName)
				return fmt.Errorf("channel %q does not own subscriber %q", channel.Name, genName)
			} else if err != nil {
				r.Recorder.Eventf(channel, corev1.EventTypeWarning, "SubscriberCreateFailed", "Creating Subscriber %q failed", genName)
				return err
			}
			r.Recorder.Eventf(channel, corev1.EventTypeNormal, "SubscriberCreated", "Created Subscriber %q", ps.Name)
		} else if !equality.Semantic.DeepEqual(ps.Spec, existingPs.Spec) {
			// Don't modify the informers copy.
			desired := existingPs.DeepCopy()
			desired.Spec = ps.Spec
			ps, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Update(desired)
			if err != nil {
				r.Recorder.Eventf(channel, corev1.EventTypeWarning, "SubscriberUpdateFailed", "Updating Subscriber %q failed", genName)
				return err
			}
			r.Recorder.Eventf(channel, corev1.EventTypeNormal, "SubscriberUpdated", "Updated Subscriber %q", ps.Name)
		}
		for i, ss := range channel.Status.SubscribableStatus.Subscribers {
			if ss.UID == s.UID {
				channel.Status.SubscribableStatus.Subscribers[i].ObservedGeneration = s.Generation
				break
			}
		}
		return nil
	}
	for _, s := range subDeletes {
		genName := resources.GenerateSubscriptionName(s.UID)
		// TODO: we need to handle the case of a already deleted pull subscription. Perhaps move to ensure deleted method.
		if err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).Delete(genName, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Desugar().Error("unable to delete PullSubscription for Channel", zap.String("ps", genName), zap.String("channel", channel.Name), zap.Error(err))
			r.Recorder.Eventf(channel, corev1.EventTypeWarning, "SubscriberDeleteFailed", "Deleting Subscriber %q failed", genName)
			return err
		}
		r.Recorder.Eventf(channel, corev1.EventTypeNormal, "SubscriberDeleted", "Deleted Subscriber %q", genName)

		for i, ss := range channel.Status.SubscribableStatus.Subscribers {
			if ss.UID == s.UID {
				// Swap len-1 with i and then pop len-1 off the slice.
				channel.Status.SubscribableStatus.Subscribers[i] = channel.Status.SubscribableStatus.Subscribers[len(channel.Status.SubscribableStatus.Subscribers)-1]
				channel.Status.SubscribableStatus.Subscribers = channel.Status.SubscribableStatus.Subscribers[:len(channel.Status.SubscribableStatus.Subscribers)-1]
				break
			}
		}
		return nil // Signal a re-reconcile.
	}

	return nil
}

func (r *Reconciler) syncSubscribersStatus(ctx context.Context, channel *v1alpha1.Channel) error {
	if channel.Status.SubscribableStatus == nil {
		channel.Status.SubscribableStatus = &eventingduck.SubscribableStatus{
			Subscribers: make([]eventingduck.SubscriberStatus, 0),
		}
	} else if channel.Status.SubscribableStatus.Subscribers == nil {
		channel.Status.SubscribableStatus.Subscribers = make([]eventingduck.SubscriberStatus, 0)
	}

	// Make a map of subscriber name to PullSubscription for lookup.
	pullsubs := make(map[string]pubsubv1alpha1.PullSubscription)
	if subs, err := r.getPullSubscriptions(ctx, channel); err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to list PullSubscriptions", zap.Error(err))
	} else {
		for _, s := range subs {
			pullsubs[resources.ExtractUIDFromSubscriptionName(s.Name)] = s
		}
	}

	for i, ss := range channel.Status.SubscribableStatus.Subscribers {
		if ps, ok := pullsubs[string(ss.UID)]; ok {
			ready, msg := r.getPullSubscriptionStatus(&ps)
			channel.Status.SubscribableStatus.Subscribers[i].Ready = ready
			channel.Status.SubscribableStatus.Subscribers[i].Message = msg
		} else {
			logging.FromContext(ctx).Desugar().Error("Failed to find status for subscriber", zap.String("uid", string(ss.UID)))
		}
	}

	return nil
}

func (r *Reconciler) createTopic(ctx context.Context, channel *v1alpha1.Channel) (*pubsubv1alpha1.Topic, error) {
	topic, err := r.getTopic(ctx, channel)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Unable to get a Topic", zap.Error(err))
		return nil, err
	}
	if topic != nil {
		if topic.Status.Address != nil {
			channel.Status.SetAddress(topic.Status.Address.URL)
		} else {
			channel.Status.SetAddress(nil)
		}
		return topic, nil
	}
	t := resources.MakeTopic(&resources.TopicArgs{
		Owner:   channel,
		Name:    resources.GeneratePublisherName(channel),
		Project: channel.Spec.Project,
		Secret:  channel.Spec.Secret,
		Topic:   resources.GenerateTopicID(channel.UID),
		Labels:  resources.GetLabels(controllerAgentName, channel.Name, string(channel.UID)),
	})

	topic, err = r.RunClientSet.PubsubV1alpha1().Topics(channel.Namespace).Create(t)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Topic", zap.Error(err))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, "TopicCreateFailed", "Failed to created Topic %q: %s", topic.Name, err.Error())
		return nil, err
	}
	r.Recorder.Eventf(channel, corev1.EventTypeNormal, "TopicCreated", "Created Topic %q", topic.Name)
	return topic, err
}

func (r *Reconciler) getTopic(ctx context.Context, channel *v1alpha1.Channel) (*pubsubv1alpha1.Topic, error) {
	name := resources.GeneratePublisherName(channel)
	topic, err := r.topicLister.Topics(channel.Namespace).Get(name)
	if err != nil {
		return nil, err
	}
	if !metav1.IsControlledBy(topic, channel) {
		channel.Status.MarkTopicNotOwned("Topic %q is owned by another resource.", name)
		return nil, fmt.Errorf("Channel: %s does not own Topic: %s", channel.Name, name)
	}
	return topic, nil
}

func (r *Reconciler) getPullSubscriptions(ctx context.Context, channel *v1alpha1.Channel) ([]pubsubv1alpha1.PullSubscription, error) {
	sl, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(channel.Namespace).List(metav1.ListOptions{
		// Use GetLabelSelector to select all PullSubscriptions related to this channel.
		LabelSelector: resources.GetLabelSelector(controllerAgentName, channel.Name, string(channel.UID)).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "Channel",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to list PullSubscriptions", zap.Error(err))
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

func (r *Reconciler) getPullSubscriptionStatus(ps *pubsubv1alpha1.PullSubscription) (corev1.ConditionStatus, string) {
	ready := corev1.ConditionTrue
	message := ""
	if !ps.Status.IsReady() {
		ready = corev1.ConditionFalse
		message = fmt.Sprintf("PullSubscription %s is not ready", ps.Name)
	}
	return ready, message
}
