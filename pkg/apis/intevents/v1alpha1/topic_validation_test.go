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

package v1alpha1

import (
	"context"
	"strings"
	"testing"

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
		orig    interface{}
		updated TopicSpec
		allowed bool
	}{
		"nil orig": {
			updated: topicSpec,
			allowed: true,
		},
		"Topic changed": {
			orig: &topicSpec,
			updated: TopicSpec{
				Secret:  topicSpec.Secret,
				Project: topicSpec.Project,
				Topic:   "updated",
			},
			allowed: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *Topic

			if tc.orig != nil {
				if spec, ok := tc.orig.(*TopicSpec); ok {
					orig = &Topic{
						Spec: *spec,
					}
				}
			}
			updated := &Topic{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
