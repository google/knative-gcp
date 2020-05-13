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

package intevents

import (
	"context"
	"fmt"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	clientset "github.com/google/knative-gcp/pkg/client/clientset/versioned"
	duck "github.com/google/knative-gcp/pkg/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/resources"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	nilPubsubableReason                         = "NilPubsubable"
	pullSubscriptionGetFailedReason             = "PullSubscriptionGetFailed"
	pullSubscriptionCreateFailedReason          = "PullSubscriptionCreateFailed"
	PullSubscriptionStatusPropagateFailedReason = "PullSubscriptionStatusPropagateFailed"
)

type PubSubBase struct {
	*reconciler.Base

	// For dealing with Topics and Pullsubscriptions
	pubsubClient clientset.Interface

	// What do we tag receive adapter as.
	receiveAdapterName string

	// What type of receive adapter to use.
	adapterType string
}

// ReconcilePubSub reconciles Topic / PullSubscription given a PubSubSpec.
// Sets the following Conditions in the Status field appropriately:
// "TopicReady", and "PullSubscriptionReady"
// Also sets the following fields in the pubsubable.Status upon success
// TopicID, ProjectID, and SinkURI
func (psb *PubSubBase) ReconcilePubSub(ctx context.Context, pubsubable duck.PubSubable, topic, resourceGroup string) (*inteventsv1alpha1.Topic, *inteventsv1alpha1.PullSubscription, error) {
	t, err := psb.reconcileTopic(ctx, pubsubable, topic)
	if err != nil {
		return t, nil, err
	}

	ps, err := psb.ReconcilePullSubscription(ctx, pubsubable, topic, resourceGroup, false)
	if err != nil {
		return t, ps, err
	}
	return t, ps, nil
}

func (psb *PubSubBase) reconcileTopic(ctx context.Context, pubsubable duck.PubSubable, topic string) (*inteventsv1alpha1.Topic, pkgreconciler.Event) {
	if pubsubable == nil {
		return nil, fmt.Errorf("nil pubsubable passed in")
	}

	name := pubsubable.GetObjectMeta().GetName()
	args := &resources.TopicArgs{
		Namespace:   pubsubable.GetObjectMeta().GetNamespace(),
		Name:        name,
		Spec:        pubsubable.PubSubSpec(),
		Owner:       pubsubable,
		Topic:       topic,
		Labels:      resources.GetLabels(psb.receiveAdapterName, name),
		Annotations: pubsubable.GetObjectMeta().GetAnnotations(),
	}
	newTopic := resources.MakeTopic(args)

	// The old and new Topics use the same, deterministic names. So delete the old one before
	// creating the new one. They cannot both be Ready=true at the same time, so by deleting the old
	// Topic, we allow the new Topic to become ready.
	err := psb.deleteOldPubSubTopicCO(ctx, pubsubable, newTopic)
	if err != nil {
		logging.FromContext(ctx).Desugar().Info("Unable to delete old Topic", zap.Error(err))
		return nil, err
	}

	topics := psb.pubsubClient.InternalV1alpha1().Topics(newTopic.Namespace)
	t, err := topics.Get(newTopic.Name, v1.GetOptions{})
	if apierrs.IsNotFound(err) {
		t, err = topics.Create(newTopic)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create Topic", zap.Any("topic", newTopic), zap.Error(err))
			return nil, fmt.Errorf("failed to create Topic: %w", err)
		}
	} else if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to get Topics", zap.Error(err))
		return nil, fmt.Errorf("failed to get Topics: %w", err)
	} else {
		// TODO Compare the actual Topic we found with the desired one and update as needed.
	}

	status := pubsubable.PubSubStatus()
	cs := pubsubable.ConditionSet()
	if err := propagateTopicStatus(t, status, cs, topic); err != nil {
		return t, err
	}

	return t, nil
}

func (psb *PubSubBase) ReconcilePullSubscription(ctx context.Context, pubsubable duck.PubSubable, topic, resourceGroup string, isPushCompatible bool) (*inteventsv1alpha1.PullSubscription, pkgreconciler.Event) {
	if pubsubable == nil {
		logging.FromContext(ctx).Desugar().Error("Nil pubsubable passed in")
		return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, nilPubsubableReason, "nil pubsubable passed in")
	}
	namespace := pubsubable.GetObjectMeta().GetNamespace()
	name := pubsubable.GetObjectMeta().GetName()
	annotations := pubsubable.GetObjectMeta().GetAnnotations()
	spec := pubsubable.PubSubSpec()
	status := pubsubable.PubSubStatus()

	cs := pubsubable.ConditionSet()

	pullSubscriptions := psb.pubsubClient.InternalV1alpha1().PullSubscriptions(namespace)
	ps, err := pullSubscriptions.Get(name, v1.GetOptions{})
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, pullSubscriptionGetFailedReason, "Getting PullSubscription failed with: %s", err.Error())
		}
		args := &resources.PullSubscriptionArgs{
			Namespace:   namespace,
			Name:        name,
			Spec:        spec,
			Owner:       pubsubable,
			Topic:       topic,
			AdapterType: psb.adapterType,
			Labels:      resources.GetLabels(psb.receiveAdapterName, name),
			Annotations: resources.GetAnnotations(annotations, resourceGroup),
		}
		if isPushCompatible {
			args.Mode = inteventsv1alpha1.ModePushCompatible
		}

		newPS := resources.MakePullSubscription(args)
		logging.FromContext(ctx).Desugar().Debug("Creating PullSubscription", zap.Any("ps", newPS))
		ps, err = pullSubscriptions.Create(newPS)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create PullSubscription", zap.Any("ps", newPS), zap.Error(err))
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, pullSubscriptionCreateFailedReason, "Creating PullSubscription failed with: %s", err.Error())
		}
	}

	// TODO Compare the actual PullSubscription we found with the desired one and update as needed.

	if err := propagatePullSubscriptionStatus(ps, status, cs); err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to propagate PullSubscription status: %s", zap.Error(err))
		return ps, pkgreconciler.NewEvent(corev1.EventTypeWarning, PullSubscriptionStatusPropagateFailedReason, "Failed to propagate PullSubscription status: %s", err.Error())
	}

	status.SinkURI = ps.Status.SinkURI

	// The old and new PullSubscriptions can co-exist without any problems. So to bias in favor of
	// double event delivery over dropped events, don't delete the old one until the new one is
	// ready.
	if ps.Status.IsReady() {
		err = psb.deleteOldPubSubPullSubscriptionCO(ctx, pubsubable, ps)
		if err != nil {
			return ps, err
		}
	}

	return ps, nil
}

func (psb *PubSubBase) deleteOldPubSubTopicCO(_ context.Context, pubsubable duck.PubSubable, t *inteventsv1alpha1.Topic) pkgreconciler.Event {
	oldT, err := psb.pubsubClient.PubsubV1alpha1().Topics(t.Namespace).Get(t.Name, v1.GetOptions{})
	if apierrs.IsNotFound(err) {
		// It doesn't exist, so there is nothing to delete.
		return nil
	} else if err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, "OldTopicGetFailed", "unable to get old Topic in the `pubsub.events.cloud.google.com` API group: %w", err)
	}
	if !v1.IsControlledBy(oldT, pubsubable.GetObjectMeta()) {
		// If this pubsubable doesn't own it, then just ignore it. Generate an event in case users
		// are interested, but do not stop reconciliation of pubsubable, nor give it a Ready=false
		// status.
		psb.Recorder.Eventf(pubsubable,
			corev1.EventTypeWarning,
			"OldTopicNotControlled",
			"old Topic '%s/%s' in the `pubsub.events.cloud.google.com` API group is not controlled by this pubsubable, so won't be deleted. Actual owners: %v",
			oldT.Namespace, oldT.Name, oldT.OwnerReferences)
		return nil
	}

	// First, to make sure the Topic is not deleted in GCP, update the Topic with a new deletion
	// policy.
	switch pp := oldT.Spec.PropagationPolicy; pp {
	case pubsubv1alpha1.TopicPolicyCreateDelete:
		c := oldT.DeepCopy()
		c.Spec.PropagationPolicy = pubsubv1alpha1.TopicPolicyCreateNoDelete
		oldT, err = psb.pubsubClient.PubsubV1alpha1().Topics(oldT.Namespace).Update(c)
		if err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, "OldTopicUpdateFailed", "unable to update propagation policy on old Topic: %w", err)
		}
	case pubsubv1alpha1.TopicPolicyCreateNoDelete:
		// Already marked for non-deletion.
		break
	case pubsubv1alpha1.TopicPolicyNoCreateNoDelete:
		// Already marked for non-deletion.
		break
	default:
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, "OldTopicUnknownPropagationPolicy", "unknown propagation policy on old Topic: %v", pp)
	}

	err = psb.pubsubClient.PubsubV1alpha1().Topics(oldT.Namespace).Delete(oldT.Name, nil)
	if err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, "OldTopicDeletionFailed", "unable to delete old Topic in the `pubsub.events.cloud.google.com` API group: %w", err)
	}
	return nil
}

func (psb *PubSubBase) deleteOldPubSubPullSubscriptionCO(_ context.Context, pubsubable duck.PubSubable, ps *inteventsv1alpha1.PullSubscription) pkgreconciler.Event {
	oldPS, err := psb.pubsubClient.PubsubV1alpha1().PullSubscriptions(ps.Namespace).Get(ps.Name, v1.GetOptions{})
	if apierrs.IsNotFound(err) {
		// It doesn't exist, so there is nothing to delete.
		return nil
	} else if err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, "OldPullSubscriptionGetFailed", "unable to get old PullSubscription in the `pubsub.events.cloud.google.com` API group: %w", err)
	}
	if !v1.IsControlledBy(oldPS, pubsubable.GetObjectMeta()) {
		// If this pubsubable doesn't own it, then just ignore it. Generate an event in case users
		// are interested, but do not stop reconciliation of pubsubable, nor give it a Ready=false
		// status.
		psb.Recorder.Eventf(pubsubable,
			corev1.EventTypeWarning,
			"oldPullSubscriptionNotControlled",
			"old PullSubscription '%s/%s' in the `pubsub.events.cloud.google.com` API group is not controlled by this pubsubable, so won't be deleted. Actual owners: %v",
			oldPS.Namespace, oldPS.Name, oldPS.OwnerReferences)
		return nil
	}
	err = psb.pubsubClient.PubsubV1alpha1().PullSubscriptions(oldPS.Namespace).Delete(oldPS.Name, nil)
	if err != nil {
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, "OldPullSubscriptionDeletionFailed", "unable to delete old PullSubscription in the `pubsub.events.cloud.google.com` API group: %w", err)
	}
	return nil
}

func propagatePullSubscriptionStatus(ps *inteventsv1alpha1.PullSubscription, status *duckv1alpha1.PubSubStatus, cs *apis.ConditionSet) error {
	pc := ps.Status.GetTopLevelCondition()
	if pc == nil {
		status.MarkPullSubscriptionNotConfigured(cs)
		return fmt.Errorf("PullSubscription %q has not yet been reconciled", ps.Name)
	}
	switch {
	case pc.Status == corev1.ConditionUnknown:
		status.MarkPullSubscriptionUnknown(cs, pc.Reason, pc.Message)
		return fmt.Errorf("the status of PullSubscription %q is Unknown", ps.Name)
	case pc.Status == corev1.ConditionTrue:
		status.MarkPullSubscriptionReady(cs)
	case pc.Status == corev1.ConditionFalse:
		status.MarkPullSubscriptionFailed(cs, pc.Reason, pc.Message)
		return fmt.Errorf("the status of PullSubscription %q is False", ps.Name)
	default:
		status.MarkPullSubscriptionUnknown(cs, "PullSubscriptionUnknown", "The status of PullSubscription is invalid: %v", pc.Status)
		return fmt.Errorf("the status of PullSubscription %q is invalid: %v", ps.Name, pc.Status)
	}
	return nil
}

func propagateTopicStatus(t *inteventsv1alpha1.Topic, status *duckv1alpha1.PubSubStatus, cs *apis.ConditionSet, topic string) error {
	tc := t.Status.GetTopLevelCondition()
	if tc == nil {
		status.MarkTopicNotConfigured(cs)
		return fmt.Errorf("Topic %q has not yet been reconciled", t.Name)
	}

	switch {
	case tc.Status == corev1.ConditionUnknown:
		status.MarkTopicUnknown(cs, tc.Reason, tc.Message)
		return fmt.Errorf("the status of Topic %q is Unknown", t.Name)
	case tc.Status == corev1.ConditionTrue:
		// When the status of Topic is ConditionTrue, break here since we also need to check the ProjectID and TopicID before we make the Topic to be Ready.
		break
	case tc.Status == corev1.ConditionFalse:
		status.MarkTopicFailed(cs, tc.Reason, tc.Message)
		return fmt.Errorf("the status of Topic %q is False", t.Name)
	default:
		status.MarkTopicUnknown(cs, "TopicUnknown", "The status of Topic is invalid: %v", tc.Status)
		return fmt.Errorf("the status of Topic %q is invalid: %v", t.Name, tc.Status)
	}
	if t.Status.ProjectID == "" {
		status.MarkTopicFailed(cs, "TopicNotReady", "Topic %q did not expose projectid", t.Name)
		return fmt.Errorf("Topic %q did not expose projectid", t.Name)
	}
	if t.Status.TopicID == "" {
		status.MarkTopicFailed(cs, "TopicNotReady", "Topic %q did not expose topicid", t.Name)
		return fmt.Errorf("Topic %q did not expose topicid", t.Name)
	}
	if t.Status.TopicID != topic {
		status.MarkTopicFailed(cs, "TopicNotReady", "Topic %q mismatch: expected %q got %q", t.Name, topic, t.Status.TopicID)
		return fmt.Errorf("Topic %q mismatch: expected %q got %q", t.Name, topic, t.Status.TopicID)
	}
	status.TopicID = t.Status.TopicID
	status.ProjectID = t.Status.ProjectID
	status.MarkTopicReady(cs)
	return nil
}

func (psb *PubSubBase) DeletePubSub(ctx context.Context, pubsubable duck.PubSubable) error {
	if pubsubable == nil {
		return fmt.Errorf("nil pubsubable passed in")
	}
	namespace := pubsubable.GetObjectMeta().GetNamespace()
	name := pubsubable.GetObjectMeta().GetName()
	status := pubsubable.PubSubStatus()
	cs := pubsubable.ConditionSet()

	// Delete the topic
	err := psb.pubsubClient.InternalV1alpha1().Topics(namespace).Delete(name, nil)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Failed to delete Topic", zap.String("name", name), zap.Error(err))
		status.MarkTopicFailed(cs, "TopicDeleteFailed", "Failed to delete Topic: %s", err.Error())
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	status.MarkTopicFailed(cs, "TopicDeleted", "Successfully deleted Topic: %s", name)
	status.TopicID = ""
	status.ProjectID = ""

	// Delete the pullsubscription
	err = psb.pubsubClient.InternalV1alpha1().PullSubscriptions(namespace).Delete(name, nil)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Failed to delete PullSubscription", zap.String("name", name), zap.Error(err))
		status.MarkPullSubscriptionFailed(cs, "PullSubscriptionDeleteFailed", "Failed to delete PullSubscription: %s", err.Error())
		return fmt.Errorf("failed to delete PullSubscription: %w", err)
	}
	status.MarkPullSubscriptionFailed(cs, "PullSubscriptionDeleted", "Successfully deleted PullSubscription: %s", name)
	status.SinkURI = nil
	return nil
}
