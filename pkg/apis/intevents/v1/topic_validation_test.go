/*
Copyright 2020 The Google LLC.

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
	"context"
	"strings"
	"testing"

	"github.com/google/knative-gcp/pkg/apis/duck"
	metadatatesting "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"

	v1 "github.com/google/knative-gcp/pkg/apis/duck/v1"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var (
	topicSpec = TopicSpec{
		Secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "secret-name",
			},
			Key: "secret-key",
		},
		Project:           "my-eventing-project",
		Topic:             "pubsub-topic",
		PropagationPolicy: TopicPolicyCreateDelete,
		EnablePublisher:   ptr.Bool(true),
	}

	topicSpecWithKSA = TopicSpec{
		IdentitySpec: v1.IdentitySpec{
			ServiceAccountName: "old-service-account",
		},
		Project: "my-eventing-project",
		Topic:   "pubsub-topic",
	}
)

func TestTopicValidation(t *testing.T) {
	tests := []struct {
		name string
		cr   resourcesemantics.GenericCRD
		want []string
	}{{
		name: "empty",
		cr: &Topic{
			Spec: TopicSpec{},
		},
		want: []string{
			"spec.propagationPolicy",
			"spec.topic",
		},
	}, {
		name: "min",
		cr: &Topic{
			Spec: TopicSpec{
				Topic:             "topic",
				PropagationPolicy: TopicPolicyCreateNoDelete,
			},
		},
		want: nil,
	}, {
		name: "invalid propagation policy",
		cr: &Topic{
			Spec: TopicSpec{
				Topic:             "topic",
				PropagationPolicy: "invalid-propagation-policy",
			},
		},
		want: []string{
			"invalid value: invalid-propagation-policy: spec.propagationPolicy",
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cr.Validate(context.TODO())

			for _, v := range test.want {
				if !strings.Contains(got.Error(), v) {
					t.Errorf("%s: validate contains (want, got) = %v, %v", test.name, v, got.Error())
				}
			}
		})
	}
}

func TestTopicCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig              interface{}
		updated           TopicSpec
		origAnnotation    map[string]string
		updatedAnnotation map[string]string
		allowed           bool
	}{
		"nil orig": {
			updated: topicSpec,
			allowed: true,
		},
		"ClusterName annotation changed": {
			origAnnotation: map[string]string{
				duck.ClusterNameAnnotation: metadatatesting.FakeClusterName + "old",
			},
			updatedAnnotation: map[string]string{
				duck.ClusterNameAnnotation: metadatatesting.FakeClusterName + "new",
			},
			allowed: false,
		},
		"AnnotationClass annotation changed": {
			origAnnotation: map[string]string{
				duck.AutoscalingClassAnnotation: duck.KEDA,
			},
			updatedAnnotation: map[string]string{
				duck.AutoscalingClassAnnotation: duck.KEDA + "new",
			},
			allowed: false,
		},
		"AnnotationClass annotation added": {
			origAnnotation: map[string]string{},
			updatedAnnotation: map[string]string{
				duck.AutoscalingClassAnnotation: duck.KEDA,
			},
			allowed: false,
		},
		"AnnotationClass annotation deleted": {
			origAnnotation: map[string]string{
				duck.AutoscalingClassAnnotation: duck.KEDA,
			},
			updatedAnnotation: map[string]string{},
			allowed:           false,
		},
		"Topic changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret:            topicSpec.Secret,
				Project:           topicSpec.Project,
				Topic:             "updated",
				PropagationPolicy: topicSpec.PropagationPolicy,
				EnablePublisher:   topicSpec.EnablePublisher,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret:            topicSpec.Secret,
				Project:           "new-project",
				Topic:             topicSpec.Topic,
				PropagationPolicy: topicSpec.PropagationPolicy,
				EnablePublisher:   topicSpec.EnablePublisher,
			},
			allowed: false,
		},
		"PropagationPolicy changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret:            topicSpec.Secret,
				Project:           "new-project",
				Topic:             topicSpec.Topic,
				PropagationPolicy: TopicPolicyCreateNoDelete,
				EnablePublisher:   topicSpec.EnablePublisher,
			},
			allowed: false,
		},
		"EnablePublisher changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret:            topicSpec.Secret,
				Project:           "new-project",
				Topic:             topicSpec.Topic,
				PropagationPolicy: topicSpec.PropagationPolicy,
				EnablePublisher:   ptr.Bool(false),
			},
		},
		"Secret.Key changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: topicSpec.Secret.Name,
					},
					Key: "some-other-key",
				},
				Project:           topicSpec.Project,
				Topic:             topicSpec.Topic,
				PropagationPolicy: topicSpec.PropagationPolicy,
				EnablePublisher:   topicSpec.EnablePublisher,
			},
			allowed: false,
		},
		"Secret.Name changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "some-other-name",
					},
					Key: topicSpec.Secret.Key,
				},
				Project:           topicSpec.Project,
				Topic:             topicSpec.Topic,
				PropagationPolicy: topicSpec.PropagationPolicy,
				EnablePublisher:   topicSpec.EnablePublisher,
			},
			allowed: false,
		},
		"ServiceAccountName changed": {
			orig: &topicSpecWithKSA,
			updated: TopicSpec{
				Project: topicSpecWithKSA.Project,
				Topic:   topicSpecWithKSA.Topic,
				IdentitySpec: v1.IdentitySpec{
					ServiceAccountName: "new-service-account",
				},
			},
			allowed: false,
		},
		"ServiceAccountName added": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: topicSpec.Secret.Name,
					},
					Key: topicSpec.Secret.Key,
				},
				Project:           topicSpec.Project,
				Topic:             topicSpec.Topic,
				PropagationPolicy: topicSpec.PropagationPolicy,
				EnablePublisher:   topicSpec.EnablePublisher,
				IdentitySpec: v1.IdentitySpec{
					ServiceAccountName: "new-service-account",
				},
			},
			allowed: false,
		},
		"ClusterName annotation added": {
			origAnnotation: nil,
			updatedAnnotation: map[string]string{
				duck.ClusterNameAnnotation: metadatatesting.FakeClusterName + "new",
			},
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *Topic

			if tc.origAnnotation != nil {
				orig = &Topic{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: tc.origAnnotation,
					},
				}
			} else if tc.orig != nil {
				if spec, ok := tc.orig.(*TopicSpec); ok {
					orig = &Topic{
						Spec: *spec,
					}
				}
			}
			updated := &Topic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.updatedAnnotation,
				},
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
