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

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

// TopicOption enables further configuration of a Topic.
type TopicOption func(*v1alpha1.Topic)

// NewTopic creates a Topic with TopicOptions
func NewTopic(name, namespace string, so ...TopicOption) *v1alpha1.Topic {
	s := &v1alpha1.Topic{
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

func WithTopicUID(uid types.UID) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.UID = uid
	}
}

// WithInitTopicConditions initializes the Topics's conditions.
func WithInitTopicConditions(s *v1alpha1.Topic) {
	s.Status.InitializeConditions()
}

func WithTopicTopicID(topicID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithTopicPropagationPolicy(policy string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Spec.PropagationPolicy = v1alpha1.PropagationPolicyType(policy)
	}
}

func WithTopicMarkTopicCreating(topicID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.MarkTopicOperating("Creating", "Created Job to create topic %q.", topicID)
		s.Status.TopicID = topicID
	}
}

func WithTopicMarkTopicVerifying(topicID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.MarkTopicOperating("Verifying", "Created Job to verify topic %q.", topicID)
		s.Status.TopicID = topicID
	}
}

func WithTopicTopicDeleting(topicID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.MarkTopicOperating("Deleting", "Created Job to delete topic %q.", topicID)
		s.Status.TopicID = topicID
	}
}

func WithTopicTopicDeleted(topicID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.MarkNoTopic("Deleted", "Successfully deleted topic %q.", topicID)
		s.Status.TopicID = ""
	}
}

func WithTopicJobFailure(topicID, reason, message string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.TopicID = topicID
		s.Status.MarkNoTopic(reason, message)
	}
}

func WithTopicAddress(uri string) TopicOption {
	return func(s *v1alpha1.Topic) {
		if uri != "" {
			u, _ := apis.ParseURL(uri)
			s.Status.SetAddress(u)
		} else {
			s.Status.SetAddress(nil)
		}
	}
}

func WithTopicSpec(spec v1alpha1.TopicSpec) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Spec = spec
	}
}

func WithTopicDeployed(s *v1alpha1.Topic) {
	s.Status.MarkDeployed()
}

func WithTopicProjectID(projectID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.ProjectID = projectID
	}
}

func WithTopicReady(topicID string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.InitializeConditions()
		s.Status.MarkDeployed()
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithTopicDeleted(s *v1alpha1.Topic) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithTopicOwnerReferences(ownerReferences []metav1.OwnerReference) TopicOption {
	return func(c *v1alpha1.Topic) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithTopicLabels(labels map[string]string) TopicOption {
	return func(c *v1alpha1.Topic) {
		c.ObjectMeta.Labels = labels
	}
}

func WithTopicFinalizers(finalizers ...string) TopicOption {
	return func(s *v1alpha1.Topic) {
		s.Finalizers = finalizers
	}
}
