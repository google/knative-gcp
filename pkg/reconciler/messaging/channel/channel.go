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

	"github.com/google/knative-gcp/pkg/reconciler/celltenant"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	channelreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/messaging/v1beta1/channel"
)

const (
	resourceGroup = "channels.messaging.cloud.google.com"

	reconciledSuccessReason           = "ChannelReconciled"
	reconciledSubscribersFailedReason = "SubscribersReconcileFailed"
)
const (
	// Name of the corev1.Events emitted from the Broker reconciliation process.
	channelReconciled = "ChannelReconciled"
	channelFinalized  = "ChannelFinalized"
)

type Reconciler struct {
	celltenant.Reconciler
	targetReconciler *celltenant.TargetReconciler
}

// Check that Reconciler implements Interface
var _ channelreconciler.Interface = (*Reconciler)(nil)
var _ channelreconciler.Finalizer = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, c *v1beta1.Channel) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling Channel", zap.Any("channel", c))
	c.Status.InitializeConditions()
	c.Status.ObservedGeneration = c.Generation

	bcs := celltenant.StatusableFromChannel(c)
	if err := r.Reconciler.ReconcileGCPCellTenant(ctx, bcs); err != nil {
		logging.FromContext(ctx).Error("Problem reconciling Channel", zap.Error(err))
		return fmt.Errorf("failed to reconcile Channel: %w", err)
		//TODO instead of returning on error, update the data plane configmap with
		// whatever info is available. or put this in a defer?
	}

	// Sync all subscriptions.
	//   a. create all subscriptions that are in spec and not in status.
	//   b. delete all subscriptions that are in status but not in spec.
	if err := r.syncSubscribers(ctx, c); err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, reconciledSubscribersFailedReason, "Reconcile Subscribers failed with: %s", err.Error())
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, channelReconciled, `Channel reconciled: "%s/%s"`, c.Namespace, c.Name)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, c *v1beta1.Channel) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Debug("Finalizing Channel", zap.Any("channel", c))
	bcs := celltenant.StatusableFromChannel(c)
	if err := r.Reconciler.FinalizeGCPCellTenant(ctx, bcs); err != nil {
		return err
	}

	if err := r.deleteAllSubscriberTopicsAndPullSubscriptions(ctx, c); err != nil {
		return err
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, channelFinalized, `Channel finalized: "%s/%s"`, c.Namespace, c.Name)
}

func (r *Reconciler) syncSubscribers(ctx context.Context, channel *v1beta1.Channel) error {
	if channel.Status.SubscribableStatus.Subscribers == nil {
		channel.Status.SubscribableStatus.Subscribers = make([]eventingduckv1beta1.SubscriberStatus, 0)
	}

	// Determine which subscribers should be deleted by getting all subscribers in the status and
	// removing those that are still in the spec.
	subDeletes := make(map[types.UID]eventingduckv1beta1.SubscriberStatus, len(channel.Status.Subscribers))
	for _, s := range channel.Status.Subscribers {
		subDeletes[s.UID] = s
	}
	if channel.Spec.SubscribableSpec != nil {
		for _, s := range channel.Spec.SubscribableSpec.Subscribers {
			delete(subDeletes, s.UID)
		}

		// Make sure all the spec Subscribers have their retry topic and subscription.
		for _, s := range channel.Spec.SubscribableSpec.Subscribers {
			t, status := celltenant.TargetFromSubscriberSpec(channel, s)
			err := r.targetReconciler.ReconcileRetryTopicAndSubscription(ctx, r.Recorder, t)
			writeSubscriberStatus(channel, s, status)
			if err != nil {
				return fmt.Errorf("unable to reconcile subscriber %q: %w", s.UID, err)
			}
		}
	}

	// Delete the no longer needed subscribers.
	for _, s := range subDeletes {
		t, _ := celltenant.TargetFromSubscriberStatus(channel, s)
		err := r.targetReconciler.DeleteRetryTopicAndSubscription(ctx, r.Recorder, t)
		if err != nil {
			return fmt.Errorf("unable to remove subscriber %q: %w", s.UID, err)
		}
		// TODO Do better than this n^2 algorithm.
		removeSubscriberStatus(channel, s)
	}

	return nil
}

func writeSubscriberStatus(channel *v1beta1.Channel, s eventingduckv1beta1.SubscriberSpec, status *celltenant.SubscriberStatus) {
	newStatus := eventingduckv1beta1.SubscriberStatus{
		UID:                s.UID,
		ObservedGeneration: s.Generation,
		Ready:              status.Ready(),
		Message:            status.Message(),
	}
	for i, ss := range channel.Status.SubscribableStatus.Subscribers {
		if ss.UID == s.UID {
			channel.Status.SubscribableStatus.Subscribers[i] = newStatus
			return
		}
	}
	// It does not yet exist, add it to the end of the slice.
	channel.Status.SubscribableStatus.Subscribers = append(channel.Status.SubscribableStatus.Subscribers, newStatus)
}

func removeSubscriberStatus(channel *v1beta1.Channel, s eventingduckv1beta1.SubscriberStatus) {
	for i, ss := range channel.Status.SubscribableStatus.Subscribers {
		if ss.UID == s.UID {
			// Remove this subscriber status by swapping len-1 with i and then popping len-1 off
			// the slice.
			channel.Status.SubscribableStatus.Subscribers[i] = channel.Status.SubscribableStatus.Subscribers[len(channel.Status.SubscribableStatus.Subscribers)-1]
			channel.Status.SubscribableStatus.Subscribers = channel.Status.SubscribableStatus.Subscribers[:len(channel.Status.SubscribableStatus.Subscribers)-1]
			return
		}
	}
}

func (r *Reconciler) deleteAllSubscriberTopicsAndPullSubscriptions(ctx context.Context, c *v1beta1.Channel) error {
	// Delete the no longer needed subscribers.
	for _, s := range c.Status.Subscribers {
		t, _ := celltenant.TargetFromSubscriberStatus(c, s)
		err := r.targetReconciler.DeleteRetryTopicAndSubscription(ctx, r.Recorder, t)
		if err != nil {
			return fmt.Errorf("unable to remove subscriber %q: %w", s.UID, err)
		}
		// TODO Do better than this n^2 algorithm.
		removeSubscriberStatus(c, s)
	}
	// TODO If we weren't able to record the status properly, there may be some Topics and
	// Subscriptions that correspond to spec subscribers. E.g. we reconciled them, but writing the
	// updated status failed, so the GCP resources were created, but they aren't in the status.
	// If we see this in practice, we should delete all the entries from spec.subscribers as well.
	return nil
}
