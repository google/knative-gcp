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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

func TestMakeTopic(t *testing.T) {
	channel := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "channel-name",
			Namespace: "channel-namespace",
		},
		Spec: v1alpha1.ChannelSpec{
			Project: "eventing-name",
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
		},
		Status: v1alpha1.ChannelStatus{
			ProjectID: "project-123",
			TopicID:   "topic-abc",
		},
	}

	got := MakeTopic(&TopicArgs{
		Owner:   channel,
		Name:    GeneratePublisherName(channel),
		Project: channel.Status.ProjectID,
		Topic:   channel.Status.TopicID,
		Secret:  channel.Spec.Secret,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
	})

	yes := true
	want := &pubsubv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "channel-namespace",
			Name:      "cre-channel-name-chan",
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "messaging.cloud.google.com/v1alpha1",
				Kind:               "Channel",
				Name:               "channel-name",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: pubsubv1alpha1.TopicSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
			Project:           "project-123",
			Topic:             "topic-abc",
			PropagationPolicy: pubsubv1alpha1.TopicPolicyCreateDelete,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
