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

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

func TestMakePullSubscription(t *testing.T) {
	channel := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "channel-name",
			Namespace: "channel-namespace",
			UID:       "channel-uid",
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

	got := MakePullSubscription(&PullSubscriptionArgs{
		Owner:   channel,
		Name:    GenerateSubscriptionName("subscriber-uid"),
		Project: channel.Status.ProjectID,
		Topic:   channel.Status.TopicID,
		Secret:  channel.Spec.Secret,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		Subscriber: duckv1alpha1.SubscriberSpec{
			SubscriberURI: "http://subscriber/",
			ReplyURI:      "http://reply/",
		},
	})

	yes := true
	want := &pubsubv1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "channel-namespace",
			Name:      "cre-sub-subscriber-uid",
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "messaging.cloud.google.com/v1alpha1",
				Kind:               "Channel",
				Name:               "channel-name",
				UID:                "channel-uid",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: pubsubv1alpha1.PullSubscriptionSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
			Project: "project-123",
			Topic:   "topic-abc",
			Sink: apisv1alpha1.Destination{
				URI: &apis.URL{Scheme: "http", Host: "reply", Path: "/"},
			},
			Transformer: &apisv1alpha1.Destination{
				URI: &apis.URL{Scheme: "http", Host: "subscriber", Path: "/"},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestMakePullSubscription_JustSubscriber(t *testing.T) {
	channel := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "channel-name",
			Namespace: "channel-namespace",
			UID:       "channel-uid",
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

	got := MakePullSubscription(&PullSubscriptionArgs{
		Owner:   channel,
		Name:    GenerateSubscriptionName("subscriber-uid"),
		Project: channel.Status.ProjectID,
		Topic:   channel.Status.TopicID,
		Secret:  channel.Spec.Secret,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		Subscriber: duckv1alpha1.SubscriberSpec{
			SubscriberURI: "http://subscriber/",
		},
	})

	yes := true
	want := &pubsubv1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "channel-namespace",
			Name:      "cre-sub-subscriber-uid",
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "messaging.cloud.google.com/v1alpha1",
				Kind:               "Channel",
				Name:               "channel-name",
				UID:                "channel-uid",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: pubsubv1alpha1.PullSubscriptionSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
			Project: "project-123",
			Topic:   "topic-abc",
			Sink: apisv1alpha1.Destination{
				URI: &apis.URL{Scheme: "http", Host: "subscriber", Path: "/"},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestMakePullSubscription_JustReply(t *testing.T) {
	channel := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "channel-name",
			Namespace: "channel-namespace",
			UID:       "channel-uid",
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

	got := MakePullSubscription(&PullSubscriptionArgs{
		Owner:   channel,
		Name:    GenerateSubscriptionName("subscriber-uid"),
		Project: channel.Status.ProjectID,
		Topic:   channel.Status.TopicID,
		Secret:  channel.Spec.Secret,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		Subscriber: duckv1alpha1.SubscriberSpec{
			ReplyURI: "http://reply/",
		},
	})

	yes := true
	want := &pubsubv1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "channel-namespace",
			Name:      "cre-sub-subscriber-uid",
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "messaging.cloud.google.com/v1alpha1",
				Kind:               "Channel",
				Name:               "channel-name",
				UID:                "channel-uid",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: pubsubv1alpha1.PullSubscriptionSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
			Project: "project-123",
			Topic:   "topic-abc",
			Sink: apisv1alpha1.Destination{
				URI: &apis.URL{Scheme: "http", Host: "reply", Path: "/"},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
