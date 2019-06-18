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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
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

func WithChannelTopic(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithChannelMarkTopicCreating(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.MarkTopicOperating("Creating", "Created Job to create Topic %q.", topicID)
		s.Status.TopicID = topicID
	}
}

func WithChannelTopicDeleting(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.MarkTopicOperating("Deleting", "Created Job to delete Topic %q.", topicID)
		s.Status.TopicID = topicID
	}
}

func WithChannelTopicDeleted(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.MarkNoTopic("Deleted", "Successfully deleted Topic %q.", topicID)
		s.Status.TopicID = ""
	}
}

func WithChannelSpec(spec v1alpha1.ChannelSpec) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Spec = spec
	}
}

func WithChannelInvokerDeployed(s *v1alpha1.Channel) {
	s.Status.MarkDeployed()
}

func WithChannelReady(topicID string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.InitializeConditions()
		s.Status.MarkDeployed()
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

//func WithChannelProjectResolved(projectID string) ChannelOption {
//	return func(s *v1alpha1.Channel) {
//		s.Status.ProjectID = projectID
//	}
//}
//
//func WithChannelSinkNotFound() ChannelOption {
//	return func(s *v1alpha1.Channel) {
//		s.Status.MarkNoSink("NotFound", "")
//	}
//}

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

func WithChannelFinalizers(finalizers ...string) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Finalizers = finalizers
	}
}
