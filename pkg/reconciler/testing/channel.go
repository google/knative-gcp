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
	"context"
	"time"

	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*v1alpha1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, so ...ChannelOption) *v1alpha1.Channel {
	s := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

// NewChannelWithoutNamespace creates a Channel with ChannelOptions but without a specific namespace
func NewChannelWithoutNamespace(name string, so ...ChannelOption) *v1alpha1.Channel {
	s := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithChannelUID(uid types.UID) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.UID = uid
	}
}

func WithChannelGenerateName(generateName string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.ObjectMeta.GenerateName = generateName
	}
}

// WithInitChannelConditions initializes the Channels's conditions.
func WithInitChannelConditions(s *v1alpha1.Channel) {
	s.Status.InitializeConditions()
}

// WithChannelServiceAccountName will give status.ServiceAccountName a k8s service account name, which is related on Workload Identity's Google service account.
func WithChannelServiceAccountName(name string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.ServiceAccountName = name
	}
}

func WithChannelWorkloadIdentityFailed(reason, message string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

func WithChannelTopic(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithChannelTopicID(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.TopicID = topicID
	}
}

func WithChannelTopicFailed(reason, message string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.MarkTopicFailed(reason, message)
	}
}

func WithChannelTopicUnknown(reason, message string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.MarkTopicUnknown(reason, message)
	}
}

func WithChannelSpec(spec v1alpha1.ChannelSpec) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Spec = spec
	}
}

func WithChannelDefaults(s *v1alpha1.Channel) {
	s.SetDefaults(context.Background())
}

func WithChannelGCPServiceAccount(gServiceAccount string) ChannelOption {
	return func(ps *v1alpha1.Channel) {
		ps.Spec.GoogleServiceAccount = gServiceAccount
	}
}

func WithChannelDeletionTimestamp(s *v1alpha1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelReady(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.InitializeConditions()
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithChannelAddress(url string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		u, _ := apis.ParseURL(url)
		s.Status.SetAddress(u)
	}
}

func WithChannelSubscribers(subscribers []duckv1alpha1.SubscriberSpec) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.Subscribable = &duckv1alpha1.Subscribable{
			Subscribers: subscribers,
		}
	}
}

func WithChannelSubscribersStatus(subscribers []duckv1alpha1.SubscriberStatus) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.SubscribableStatus = &duckv1alpha1.SubscribableStatus{
			Subscribers: subscribers,
		}
	}
}

func WithChannelDeleted(s *v1alpha1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelOwnerReferences(ownerReferences []metav1.OwnerReference) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithChannelLabels(labels map[string]string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.ObjectMeta.Labels = labels
	}
}
