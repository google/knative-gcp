/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"time"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*v1beta1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, so ...ChannelOption) *v1beta1.Channel {
	s := &v1beta1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

// NewChannelWithoutNamespace creates a Channel with ChannelOptions but without a specific namespace
func NewChannelWithoutNamespace(name string, co ...ChannelOption) *v1beta1.Channel {
	c := &v1beta1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range co {
		opt(c)
	}
	c.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
	return c
}

func WithChannelUID(uid types.UID) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.UID = uid
	}
}

func WithChannelGenerateName(generateName string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.ObjectMeta.GenerateName = generateName
	}
}

// WithInitChannelConditions initializes the Channels's conditions.
func WithInitChannelConditions(c *v1beta1.Channel) {
	c.Status.InitializeConditions()
}

func WithChannelWorkloadIdentityFailed(reason, message string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.MarkWorkloadIdentityFailed(c.ConditionSet(), reason, message)
	}
}

func WithChannelTopic(topicID string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.MarkTopicReady()
		c.Status.TopicID = topicID
	}
}

func WithChannelTopicID(topicID string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.TopicID = topicID
	}
}

func WithChannelTopicFailed(reason, message string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.MarkTopicFailed(reason, message)
	}
}

func WithChannelTopicUnknown(reason, message string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.MarkTopicUnknown(reason, message)
	}
}

func WithChannelSpec(spec v1beta1.ChannelSpec) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Spec = spec
	}
}

func WithChannelSetDefaults(c *v1beta1.Channel) {
	c.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
}

func WithChannelServiceAccount(kServiceAccount string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Spec.ServiceAccountName = kServiceAccount
	}
}

func WithChannelDeletionTimestamp(c *v1beta1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelReady(topicID string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.InitializeConditions()
		c.Status.MarkTopicReady()
		c.Status.TopicID = topicID
	}
}

func WithChannelAddress(url string) ChannelOption {
	return func(c *v1beta1.Channel) {
		u, _ := apis.ParseURL(url)
		c.Status.SetAddress(u)
	}
}

func WithChannelSubscribers(subscribers []duckv1beta1.SubscriberSpec) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Spec.SubscribableSpec = &duckv1beta1.SubscribableSpec{
			Subscribers: subscribers,
		}
	}
}

func WithChannelSubscribersStatus(subscribers []eventingduckv1beta1.SubscriberStatus) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.SubscribableStatus = duckv1beta1.SubscribableStatus{
			Subscribers: subscribers,
		}
	}
}

func WithChannelDeleted(s *v1beta1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelOwnerReferences(ownerReferences []metav1.OwnerReference) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithChannelLabels(labels map[string]string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.ObjectMeta.Labels = labels
	}
}

func WithChannelAnnotations(Annotations map[string]string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.ObjectMeta.Annotations = Annotations
	}
}
