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

// PubSubTopicOption enables further configuration of a Topic.
type PubSubTopicOption func(*v1alpha1.Topic)

// NewPubSubTopic creates a Topic with TopicOptions
func NewPubSubTopic(name, namespace string, so ...PubSubTopicOption) *v1alpha1.Topic {
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

func WithPubSubTopicUID(uid types.UID) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.UID = uid
	}
}

// WithPubSubInitTopicConditions initializes the Topics's conditions.
func WithPubSubInitTopicConditions(s *v1alpha1.Topic) {
	s.Status.InitializeConditions()
}

func WithPubSubTopicTopicID(topicID string) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithPubSubTopicPropagationPolicy(policy string) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Spec.PropagationPolicy = v1alpha1.PropagationPolicyType(policy)
	}
}

func WithPubSubTopicAddress(uri string) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		if uri != "" {
			u, _ := apis.ParseURL(uri)
			s.Status.SetAddress(u)
		} else {
			s.Status.SetAddress(nil)
		}
	}
}

func WithPubSubTopicSpec(spec v1alpha1.TopicSpec) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Spec = spec
	}
}

func WithPubSubTopicPublisherDeployed(s *v1alpha1.Topic) {
	s.Status.MarkPublisherDeployed()
}

func WithPubSubTopicPublisherNotDeployed(reason, message string) PubSubTopicOption {
	return func(t *v1alpha1.Topic) {
		t.Status.MarkPublisherNotDeployed(reason, message)
	}
}

func WithPubSubTopicPublisherUnknown(reason, message string) PubSubTopicOption {
	return func(t *v1alpha1.Topic) {
		t.Status.MarkPublisherUnknown(reason, message)
	}
}

func WithPubSubTopicPublisherNotConfigured() PubSubTopicOption {
	return func(t *v1alpha1.Topic) {
		t.Status.MarkPublisherNotConfigured()
	}
}

func WithPubSubTopicProjectID(projectID string) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.ProjectID = projectID
	}
}

func WithPubSubTopicReady(topicID string) PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.InitializeConditions()
		s.Status.MarkPublisherDeployed()
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithPubSubTopicFailed() PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.InitializeConditions()
		s.Status.MarkPublisherNotDeployed("PublisherStatus", "Publisher has no Ready type status")
	}
}

func WithPubSubTopicUnknown() PubSubTopicOption {
	return func(s *v1alpha1.Topic) {
		s.Status.InitializeConditions()
	}
}

func WithPubSubTopicDeleted(t *v1alpha1.Topic) {
	tt := metav1.NewTime(time.Unix(1e9, 0))
	t.ObjectMeta.SetDeletionTimestamp(&tt)
}

func WithPubSubTopicOwnerReferences(ownerReferences []metav1.OwnerReference) PubSubTopicOption {
	return func(c *v1alpha1.Topic) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithPubSubTopicLabels(labels map[string]string) PubSubTopicOption {
	return func(c *v1alpha1.Topic) {
		c.ObjectMeta.Labels = labels
	}
}

func WithPubSubTopicNoTopic(reason, message string) PubSubTopicOption {
	return func(t *v1alpha1.Topic) {
		t.Status.MarkNoTopic(reason, message)
	}
}

func WithPubSubTopicAnnotations(annotations map[string]string) PubSubTopicOption {
	return func(c *v1alpha1.Topic) {
		c.ObjectMeta.Annotations = annotations
	}
}
