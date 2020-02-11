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

package pubsub

import (
	"context"
	"fmt"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	clientset "github.com/google/knative-gcp/pkg/client/clientset/versioned"
	"github.com/google/knative-gcp/pkg/duck"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
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
func (psb *PubSubBase) ReconcilePubSub(ctx context.Context, pubsubable duck.PubSubable, topic, resourceGroup string) (*pubsubv1alpha1.Topic, *pubsubv1alpha1.PullSubscription, error) {
	if pubsubable == nil {
		return nil, nil, fmt.Errorf("nil pubsubable passed in")
	}
	namespace := pubsubable.GetObjectMeta().GetNamespace()
	name := pubsubable.GetObjectMeta().GetName()
	spec := pubsubable.PubSubSpec()
	status := pubsubable.PubSubStatus()

	topics := psb.pubsubClient.PubsubV1alpha1().Topics(namespace)
	t, err := topics.Get(name, v1.GetOptions{})

	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get Topics", zap.Error(err))
			return nil, nil, fmt.Errorf("failed to get Topics: %w", err)
		}
		args := &resources.TopicArgs{
			Namespace: namespace,
			Name:      name,
			Spec:      spec,
			Owner:     pubsubable,
			Topic:     topic,
			Labels:    resources.GetLabels(psb.receiveAdapterName, name),
		}
		newTopic := resources.MakeTopic(args)
		t, err = topics.Create(newTopic)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create Topic", zap.Any("topic", newTopic), zap.Error(err))
			return nil, nil, fmt.Errorf("failed to create Topic: %w", err)
		}
	}

	cs := pubsubable.ConditionSet()

	if err := propagateTopicStatus(t, status, cs, topic); err != nil {
		return t, nil, err
	}

	// Ok, so the Topic is ready, let's reconcile PullSubscription.
	pullSubscriptions := psb.pubsubClient.PubsubV1alpha1().PullSubscriptions(namespace)
	ps, err := pullSubscriptions.Get(name, v1.GetOptions{})
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return t, nil, fmt.Errorf("failed to get Pullsubscription: %w", err)
		}
		args := &resources.PullSubscriptionArgs{
			Namespace:     namespace,
			Name:          name,
			Spec:          spec,
			Owner:         pubsubable,
			Topic:         topic,
			AdapterType:   psb.adapterType,
			ResourceGroup: resourceGroup,
			Labels:        resources.GetLabels(psb.receiveAdapterName, name),
		}
		newPS := resources.MakePullSubscription(args)
		ps, err = pullSubscriptions.Create(newPS)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create PullSubscription", zap.Any("ps", newPS), zap.Error(err))
			return t, nil, fmt.Errorf("failed to create PullSubscription: %w", err)
		}
	}

	if err := propagatePullSubscriptionStatus(ps, status, cs); err != nil {
		return t, ps, err
	}

	uri, err := apis.ParseURL(ps.Status.SinkURI)
	if err != nil {
		return t, ps, fmt.Errorf("failed to parse url %q: %w", ps.Status.SinkURI, err)
	}
	status.SinkURI = uri
	return t, ps, nil
}

func propagatePullSubscriptionStatus(ps *pubsubv1alpha1.PullSubscription, status *duckv1alpha1.PubSubStatus, cs *apis.ConditionSet) error {
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

func propagateTopicStatus(t *pubsubv1alpha1.Topic, status *duckv1alpha1.PubSubStatus, cs *apis.ConditionSet, topic string) error {
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
	err := psb.pubsubClient.PubsubV1alpha1().Topics(namespace).Delete(name, nil)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Failed to delete Topic", zap.String("name", name), zap.Error(err))
		status.MarkTopicFailed(cs, "TopicDeleteFailed", "Failed to delete Topic: %s", err.Error())
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	status.MarkTopicFailed(cs, "TopicDeleted", "Successfully deleted Topic: %s", name)
	status.TopicID = ""
	status.ProjectID = ""

	// Delete the pullsubscription
	err = psb.pubsubClient.PubsubV1alpha1().PullSubscriptions(namespace).Delete(name, nil)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Failed to delete PullSubscription", zap.String("name", name), zap.Error(err))
		status.MarkPullSubscriptionFailed(cs, "PullSubscriptionDeleteFailed", "Failed to delete PullSubscription: %s", err.Error())
		return fmt.Errorf("failed to delete PullSubscription: %w", err)
	}
	status.MarkPullSubscriptionFailed(cs, "PullSubscriptionDeleted", "Successfully deleted PullSubscription: %s", name)
	status.SinkURI = nil
	return nil
}
