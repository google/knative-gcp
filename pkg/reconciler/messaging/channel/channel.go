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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/duck"
	inteventsv1beta1 "github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	channelreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/messaging/v1beta1/channel"
	inteventslisters "github.com/google/knative-gcp/pkg/client/listers/intevents/v1beta1"
	listers "github.com/google/knative-gcp/pkg/client/listers/messaging/v1beta1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/messaging/channel/resources"
)

const (
	resourceGroup = "channels.messaging.cloud.google.com"

	reconciledSuccessReason                 = "ChannelReconciled"
	reconciledTopicFailedReason             = "TopicReconcileFailed"
	deleteWorkloadIdentityFailed            = "WorkloadIdentityDeleteFailed"
	reconciledSubscribersFailedReason       = "SubscribersReconcileFailed"
	reconciledSubscribersStatusFailedReason = "SubscribersStatusReconcileFailed"
	workloadIdentityFailed                  = "WorkloadIdentityReconcileFailed"
)

// Reconciler implements controller.Reconciler for Channel resources.
type Reconciler struct {
	*reconciler.Base
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// listers index properties about resources
	channelLister listers.ChannelLister
	topicLister   inteventslisters.TopicLister
}

// Check that our Reconciler implements Interface.
var _ channelreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, channel *v1beta1.Channel) pkgreconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("channel", channel)))

	channel.Status.InitializeConditions()
	channel.Status.ObservedGeneration = channel.Generation

	// If ServiceAccountName is provided, reconcile workload identity.
	if channel.Spec.ServiceAccountName != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, channel.Spec.Project, channel); err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile Channel workload identity: %s", err.Error())
		}
	}

	// 1. Create the Topic.
	topic, err := r.reconcileTopic(ctx, channel)
	if err != nil {
		channel.Status.MarkTopicFailed("TopicReconcileFailed", "Failed to reconcile Topic: %s", err.Error())
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, reconciledTopicFailedReason, "Reconcile Topic failed with: %s", err.Error())
	}
	channel.Status.PropagateTopicStatus(&topic.Status)
	channel.Status.TopicID = topic.Spec.Topic

	// 2. Sync all subscriptions.
	//   a. create all subscriptions that are in spec and not in status.
	//   b. delete all subscriptions that are in status but not in spec.
	if err := r.syncSubscribers(ctx, channel); err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, reconciledSubscribersFailedReason, "Reconcile Subscribers failed with: %s", err.Error())
	}

	// 3. Sync all subscriptions statuses.
	if err := r.syncSubscribersStatus(ctx, channel); err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, reconciledSubscribersStatusFailedReason, "Reconcile Subscribers Status failed with: %s", err.Error())
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, channel.Namespace, channel.Name)
}

func (r *Reconciler) syncSubscribers(ctx context.Context, channel *v1beta1.Channel) error {
	if channel.Status.SubscribableStatus.Subscribers == nil {
		channel.Status.SubscribableStatus.Subscribers = make([]eventingduckv1beta1.SubscriberStatus, 0)
	}

	subCreates := []eventingduckv1beta1.SubscriberSpec(nil)
	subUpdates := []eventingduckv1beta1.SubscriberSpec(nil)
	subDeletes := []eventingduckv1beta1.SubscriberStatus(nil)

	// Make a map of name to PullSubscription for lookup.
	pullsubs := make(map[string]inteventsv1beta1.PullSubscription)
	if subs, err := r.getPullSubscriptions(ctx, channel); err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to list PullSubscriptions", zap.Error(err))
	} else {
		for _, s := range subs {
			pullsubs[s.Name] = s
		}
	}

	exists := make(map[types.UID]eventingduckv1beta1.SubscriberStatus)
	for _, s := range channel.Status.SubscribableStatus.Subscribers {
		exists[s.UID] = s
	}

	if channel.Spec.SubscribableSpec != nil {
		for _, want := range channel.Spec.SubscribableSpec.Subscribers {
			if got, ok := exists[want.UID]; !ok {
				// If it does not exist, then create it.
				subCreates = append(subCreates, want)
			} else {
				_, found := pullsubs[resources.GeneratePullSubscriptionName(want.UID)]
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

	clusterName := channel.GetAnnotations()[duck.ClusterNameAnnotation]
	for _, s := range subCreates {
		genName := resources.GeneratePullSubscriptionName(s.UID)

		ps := resources.MakePullSubscription(&resources.PullSubscriptionArgs{
			Owner:              channel,
			Name:               genName,
			Project:            channel.Spec.Project,
			Topic:              channel.Status.TopicID,
			ServiceAccountName: channel.Spec.ServiceAccountName,
			Secret:             channel.Spec.Secret,
			Labels:             resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, genName, string(channel.UID)),
			Annotations:        resources.GetPullSubscriptionAnnotations(channel.Name, clusterName),
			Subscriber:         s,
		})
		ps, err := r.RunClientSet.InternalV1beta1().PullSubscriptions(channel.Namespace).Create(ctx, ps, metav1.CreateOptions{})
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

		channel.Status.SubscribableStatus.Subscribers = append(channel.Status.SubscribableStatus.Subscribers, eventingduckv1beta1.SubscriberStatus{
			UID:                s.UID,
			ObservedGeneration: s.Generation,
		})
		return nil // Signal a re-reconcile.
	}
	for _, s := range subUpdates {
		genName := resources.GeneratePullSubscriptionName(s.UID)

		ps := resources.MakePullSubscription(&resources.PullSubscriptionArgs{
			Owner:              channel,
			Name:               genName,
			Project:            channel.Spec.Project,
			Topic:              channel.Status.TopicID,
			ServiceAccountName: channel.Spec.ServiceAccountName,
			Secret:             channel.Spec.Secret,
			Labels:             resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, genName, string(channel.UID)),
			Annotations:        resources.GetPullSubscriptionAnnotations(channel.Name, clusterName),
			Subscriber:         s,
		})

		existingPs, found := pullsubs[genName]
		if !found {
			// PullSubscription does not exist, that's ok, create it now.
			ps, err := r.RunClientSet.InternalV1beta1().PullSubscriptions(channel.Namespace).Create(ctx, ps, metav1.CreateOptions{})
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
			ps, err := r.RunClientSet.InternalV1beta1().PullSubscriptions(channel.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
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
		genName := resources.GeneratePullSubscriptionName(s.UID)
		// TODO: we need to handle the case of a already deleted pull subscription. Perhaps move to ensure deleted method.
		if err := r.RunClientSet.InternalV1beta1().PullSubscriptions(channel.Namespace).Delete(ctx, genName, metav1.DeleteOptions{}); err != nil {
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

func (r *Reconciler) syncSubscribersStatus(ctx context.Context, channel *v1beta1.Channel) error {
	if channel.Status.SubscribableStatus.Subscribers == nil {
		channel.Status.SubscribableStatus.Subscribers = make([]eventingduckv1beta1.SubscriberStatus, 0)
	}

	// Make a map of subscriber name to PullSubscription for lookup.
	pullsubs := make(map[string]inteventsv1beta1.PullSubscription)
	if subs, err := r.getPullSubscriptions(ctx, channel); err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to list PullSubscriptions", zap.Error(err))
	} else {
		for _, s := range subs {
			pullsubs[resources.ExtractUIDFromPullSubscriptionName(s.Name)] = s
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

func (r *Reconciler) reconcileTopic(ctx context.Context, channel *v1beta1.Channel) (*inteventsv1beta1.Topic, error) {
	clusterName := channel.GetAnnotations()[duck.ClusterNameAnnotation]
	name := resources.GeneratePublisherName(channel)
	t := resources.MakeTopic(&resources.TopicArgs{
		Owner:              channel,
		Name:               name,
		Project:            channel.Spec.Project,
		ServiceAccountName: channel.Spec.ServiceAccountName,
		Secret:             channel.Spec.Secret,
		Topic:              resources.GenerateTopicID(channel),
		Labels:             resources.GetLabels(controllerAgentName, channel.Name, string(channel.UID)),
		Annotations:        resources.GetTopicAnnotations(clusterName),
	})

	topic, err := r.getTopic(ctx, channel)
	if apierrs.IsNotFound(err) {
		topic, err = r.RunClientSet.InternalV1beta1().Topics(channel.Namespace).Create(ctx, t, metav1.CreateOptions{})
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create Topic", zap.Error(err))
			r.Recorder.Eventf(channel, corev1.EventTypeWarning, "TopicCreateFailed", "Failed to created Topic %q: %s", topic.Name, err.Error())
			return nil, err
		}
		r.Recorder.Eventf(channel, corev1.EventTypeNormal, "TopicCreated", "Created Topic %q", topic.Name)
		return topic, nil
	} else if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to get Topic", zap.Error(err))
		return nil, fmt.Errorf("failed to get Topic: %w", err)
	} else if !metav1.IsControlledBy(topic, channel) {
		channel.Status.MarkTopicNotOwned("Topic %q is owned by another resource.", name)
		return nil, fmt.Errorf("Channel: %s does not own Topic: %s", channel.Name, name)
	} else if !equality.Semantic.DeepDerivative(t.Spec, topic.Spec) {
		// Don't modify the informers copy.
		desired := topic.DeepCopy()
		desired.Spec = t.Spec
		logging.FromContext(ctx).Desugar().Debug("Updating Topic", zap.Any("topic", desired))
		t, err = r.RunClientSet.InternalV1beta1().Topics(channel.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update Topic", zap.Any("topic", topic), zap.Error(err))
			return nil, fmt.Errorf("failed to update Topic: %w", err)
		}
		return t, nil
	}

	if topic != nil {
		if topic.Status.Address != nil {
			channel.Status.SetAddress(topic.Status.Address.URL)
		} else {
			channel.Status.SetAddress(nil)
		}
	}

	return topic, nil
}

func (r *Reconciler) getTopic(_ context.Context, channel *v1beta1.Channel) (*inteventsv1beta1.Topic, error) {
	name := resources.GeneratePublisherName(channel)
	topic, err := r.topicLister.Topics(channel.Namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return topic, nil
}

func (r *Reconciler) getPullSubscriptions(ctx context.Context, channel *v1beta1.Channel) ([]inteventsv1beta1.PullSubscription, error) {
	sl, err := r.RunClientSet.InternalV1beta1().PullSubscriptions(channel.Namespace).List(ctx, metav1.ListOptions{
		// Use GetLabelSelector to select all PullSubscriptions related to this channel.
		LabelSelector: resources.GetLabelSelector(controllerAgentName, channel.Name, string(channel.UID)).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1beta1.SchemeGroupVersion.String(),
			Kind:       "Channel",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to list PullSubscriptions", zap.Error(err))
		return nil, err
	}
	subs := []inteventsv1beta1.PullSubscription(nil)
	for _, subscription := range sl.Items {
		if metav1.IsControlledBy(&subscription, channel) {
			subs = append(subs, subscription)
		}
	}
	return subs, nil
}

func (r *Reconciler) getPullSubscriptionStatus(ps *inteventsv1beta1.PullSubscription) (corev1.ConditionStatus, string) {
	ready := corev1.ConditionTrue
	message := ""
	if !ps.Status.IsReady() {
		ready = corev1.ConditionFalse
		message = fmt.Sprintf("PullSubscription %s is not ready", ps.Name)
	}
	return ready, message
}

func (r *Reconciler) FinalizeKind(ctx context.Context, channel *v1beta1.Channel) pkgreconciler.Event {
	// If k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if channel.Spec.ServiceAccountName != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, channel.Spec.Project, channel); err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete Channel workload identity: %s", err.Error())
		}
	}

	return nil
}
