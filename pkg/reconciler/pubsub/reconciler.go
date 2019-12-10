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

	pubsubsourcev1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	pubsubsourceclientset "github.com/google/knative-gcp/pkg/client/clientset/versioned"
	"github.com/google/knative-gcp/pkg/duck"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

type PubSubBase struct {
	*reconciler.Base

	// For dealing with Topics and Pullsubscriptions
	pubsubClient pubsubsourceclientset.Interface

	// What do we tag receive adapter as.
	receiveAdapterName string
}

// ReconcilePubSub reconciles Topic / PullSubscription given a PubSubSpec.
// Sets the following Conditions in the Status field appropriately:
// "TopicReady", and "PullSubscriptionReady"
// Also sets the following fields in the pubsubable.Status upon success
// TopicID, ProjectID, and SinkURI
func (psb *PubSubBase) ReconcilePubSub(ctx context.Context, pubsubable duck.PubSubable, topic, resourceGroup string) (*pubsubsourcev1alpha1.Topic, *pubsubsourcev1alpha1.PullSubscription, error) {
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
			return nil, nil, fmt.Errorf("failed to get Topics: %s", err.Error())
		}
		newTopic := resources.MakeTopic(namespace, name, spec, pubsubable, topic, psb.receiveAdapterName)
		t, err = topics.Create(newTopic)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create Topic", zap.Any("topic", newTopic), zap.Error(err))
			return nil, nil, fmt.Errorf("failed to create Topic: %s", err.Error())
		}
	}

	cs := pubsubable.ConditionSet()
	if !t.Status.IsReady() {
		status.MarkTopicNotReady(cs, "TopicNotReady", "Topic %q not ready", t.Name)
		return t, nil, fmt.Errorf("Topic %q not ready", t.Name)
	}

	if t.Status.ProjectID == "" {
		status.MarkTopicNotReady(cs, "TopicNotReady", "Topic %q did not expose projectid", t.Name)
		return t, nil, fmt.Errorf("Topic %q did not expose projectid", t.Name)
	}

	if t.Status.TopicID == "" {
		status.MarkTopicNotReady(cs, "TopicNotReady", "Topic %q did not expose topicid", t.Name)
		return t, nil, fmt.Errorf("Topic %q did not expose topicid", t.Name)
	}

	if t.Status.TopicID != topic {
		status.MarkTopicNotReady(cs, "TopicNotReady", "Topic %q mismatch: expected %q got %q", t.Name, topic, t.Status.TopicID)
		return t, nil, fmt.Errorf("Topic %q mismatch: expected %q got %q", t.Name, topic, t.Status.TopicID)
	}

	status.TopicID = t.Status.TopicID
	status.ProjectID = t.Status.ProjectID
	status.MarkTopicReady(cs)

	// Ok, so the Topic is ready, let's reconcile PullSubscription.
	pullSubscriptions := psb.pubsubClient.PubsubV1alpha1().PullSubscriptions(namespace)
	ps, err := pullSubscriptions.Get(name, v1.GetOptions{})
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return t, nil, fmt.Errorf("failed to get Pullsubscription: %s", err.Error())
		}
		newPS := resources.MakePullSubscription(namespace, name, spec, pubsubable, topic, psb.receiveAdapterName, resourceGroup)
		ps, err = pullSubscriptions.Create(newPS)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create PullSubscription", zap.Any("ps", newPS), zap.Error(err))
			return t, nil, fmt.Errorf("failed to create PullSubscription: %s", err.Error())
		}
	}

	if !ps.Status.IsReady() {
		status.MarkPullSubscriptionNotReady(cs, "PullSubscriptionNotReady", "PullSubscription %q not ready", ps.Name)
		return t, ps, fmt.Errorf("PullSubscription %q not ready", ps.Name)
	} else {
		status.MarkPullSubscriptionReady(cs)
	}
	uri, err := apis.ParseURL(ps.Status.SinkURI)
	if err != nil {
		return t, ps, fmt.Errorf("failed to parse url %q: %s", ps.Status.SinkURI, err.Error())
	}
	status.SinkURI = uri
	return t, ps, nil
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
	topics := psb.pubsubClient.PubsubV1alpha1().Topics(namespace)
	err := topics.Delete(name, nil)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Failed to delete Topic", zap.String("name", name), zap.Error(err))
		status.MarkTopicNotReady(cs, "TopicDeleteFailed", "Failed to delete Topic: %s", err.Error())
		return fmt.Errorf("failed to delete topic: %s", err.Error())
	}
	status.MarkTopicNotReady(cs, "TopicDeleted", "Successfully deleted Topic: %s", name)
	status.TopicID = ""
	status.ProjectID = ""

	// Delete the pullsubscription
	pullSubscriptions := psb.pubsubClient.PubsubV1alpha1().PullSubscriptions(namespace)
	err = pullSubscriptions.Delete(name, nil)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Failed to delete PullSubscription", zap.String("name", name), zap.Error(err))
		status.MarkPullSubscriptionNotReady(cs, "PullSubscriptionDeleteFailed", "Failed to delete PullSubscription: %s", err.Error())
		return fmt.Errorf("failed to delete PullSubscription: %s", err.Error())
	}
	status.MarkPullSubscriptionNotReady(cs, "PullSubscriptionDeleted", "Successfully deleted PullSubscription: %s", name)
	status.SinkURI = nil
	return nil
}
