/*
Copyright 2020 Google LLC.

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

package v1

import (
	"time"

	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
)

// TopicOption enables further configuration of a Topic.
type TopicOption func(*v1.Topic)

// NewTopic creates a Topic with TopicOptions
func NewTopic(name, namespace string, to ...TopicOption) *v1.Topic {
	t := &v1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range to {
		opt(t)
	}
	return t
}

func WithTopicUID(uid types.UID) TopicOption {
	return func(t *v1.Topic) {
		t.UID = uid
	}
}

// WithInitTopicConditions initializes the Topics's conditions.
func WithInitTopicConditions(t *v1.Topic) {
	t.Status.InitializeConditions()
}

func WithTopicTopicID(topicID string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.MarkTopicReady()
		t.Status.TopicID = topicID
	}
}

func WithTopicPropagationPolicy(policy string) TopicOption {
	return func(t *v1.Topic) {
		t.Spec.PropagationPolicy = v1.PropagationPolicyType(policy)
	}
}

func WithTopicTopicDeleted(topicID string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.MarkNoTopic("Deleted", "Successfully deleted topic %q.", topicID)
		t.Status.TopicID = ""
	}
}

func WithTopicJobFailure(topicID, reason, message string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.TopicID = topicID
		t.Status.MarkNoTopic(reason, message)
	}
}

func WithTopicAddress(uri string) TopicOption {
	return func(t *v1.Topic) {
		if uri != "" {
			u, _ := apis.ParseURL(uri)
			t.Status.SetAddress(u)
		} else {
			t.Status.SetAddress(nil)
		}
	}
}

func WithTopicSpec(spec v1.TopicSpec) TopicOption {
	return func(t *v1.Topic) {
		t.Spec = spec
	}
}

func WithTopicPublisherDeployed(t *v1.Topic) {
	t.Status.MarkPublisherDeployed()
}

func WithTopicPublisherNotDeployed(reason, message string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.MarkPublisherNotDeployed(reason, message)
	}
}

func WithTopicPublisherUnknown(reason, message string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.MarkPublisherUnknown(reason, message)
	}
}

func WithTopicPublisherNotConfigured(t *v1.Topic) {
	t.Status.MarkPublisherNotConfigured()
}

func WithTopicProjectID(projectID string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.ProjectID = projectID
	}
}

func WithTopicReady(topicID string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.InitializeConditions()
		t.Status.MarkTopicReady()
		t.Status.TopicID = topicID
	}
}

func WithTopicReadyAndPublisherDeployed(topicID string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.InitializeConditions()
		t.Status.MarkPublisherDeployed()
		t.Status.MarkTopicReady()
		t.Status.TopicID = topicID
	}
}

func WithTopicFailed(t *v1.Topic) {
	t.Status.InitializeConditions()
	t.Status.MarkNoTopic("TopicFailed", "test message")
}

func WithTopicUnknown(t *v1.Topic) {
	t.Status.InitializeConditions()
}

func WithTopicDeleted(t *v1.Topic) {
	tt := metav1.NewTime(time.Unix(1e9, 0))
	t.ObjectMeta.SetDeletionTimestamp(&tt)
}

func WithTopicOwnerReferences(ownerReferences []metav1.OwnerReference) TopicOption {
	return func(t *v1.Topic) {
		t.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithTopicLabels(labels map[string]string) TopicOption {
	return func(t *v1.Topic) {
		t.ObjectMeta.Labels = labels
	}
}

func WithTopicNoTopic(reason, message string) TopicOption {
	return func(t *v1.Topic) {
		t.Status.MarkNoTopic(reason, message)
	}
}

func WithTopicFinalizers(finalizers ...string) TopicOption {
	return func(t *v1.Topic) {
		t.Finalizers = finalizers
	}
}

func WithTopicAnnotations(annotations map[string]string) TopicOption {
	return func(t *v1.Topic) {
		t.ObjectMeta.Annotations = annotations
	}
}

func WithTopicSetDefaults(t *v1.Topic) {
	t.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
}
