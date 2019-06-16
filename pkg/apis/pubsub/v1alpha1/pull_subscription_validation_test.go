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

package v1alpha1

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

var (
	fullSpec = PullSubscriptionSpec{
		Secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "secret-name",
			},
			Key: "secret-key",
		},
		Project: "my-eventing-project",
		Topic:   "pubsub-topic",
		Sink: &corev1.ObjectReference{
			APIVersion: "foo",
			Kind:       "bar",
			Namespace:  "baz",
			Name:       "qux",
		},
		ServiceAccountName: "service-account-name",
	}
)

func TestPubSubCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated PullSubscriptionSpec
		allowed bool
	}{
		"nil orig": {
			updated: fullSpec,
			allowed: true,
		},
		"Secret.Name changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "some-other-name",
					},
					Key: fullSpec.Secret.Key,
				},
				Project:            fullSpec.Project,
				Topic:              fullSpec.Topic,
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: "some-other-key",
				},
				Project:            fullSpec.Project,
				Topic:              fullSpec.Topic,
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project:            "some-other-project",
				Topic:              fullSpec.Topic,
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Topic changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project:            fullSpec.Project,
				Topic:              "some-other-topic",
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project: fullSpec.Project,
				Topic:   fullSpec.Topic,
				Sink: &corev1.ObjectReference{
					APIVersion: "some-other-api-version",
					Kind:       fullSpec.Sink.Kind,
					Namespace:  fullSpec.Sink.Namespace,
					Name:       fullSpec.Sink.Name,
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Kind changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project: fullSpec.Project,
				Topic:   fullSpec.Topic,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       "some-other-kind",
					Namespace:  fullSpec.Sink.Namespace,
					Name:       fullSpec.Sink.Name,
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Namespace changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project: fullSpec.Project,
				Topic:   fullSpec.Topic,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       fullSpec.Sink.Kind,
					Namespace:  "some-other-namespace",
					Name:       fullSpec.Sink.Name,
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Name changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project: fullSpec.Project,
				Topic:   fullSpec.Topic,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       fullSpec.Sink.Kind,
					Namespace:  fullSpec.Sink.Namespace,
					Name:       "some-other-name",
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"ServiceAccountName changed": {
			orig: &fullSpec,
			updated: PullSubscriptionSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fullSpec.Secret.Name,
					},
					Key: fullSpec.Secret.Key,
				},
				Project: fullSpec.Project,
				Topic:   fullSpec.Topic,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       fullSpec.Sink.Kind,
					Namespace:  fullSpec.Sink.Namespace,
					Name:       "some-other-name",
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"no change": {
			orig:    &fullSpec,
			updated: fullSpec,
			allowed: true,
		},
		"not spec": {
			orig:    []string{"wrong"},
			updated: fullSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *PullSubscription

			if tc.orig != nil {
				if spec, ok := tc.orig.(*PullSubscriptionSpec); ok {
					orig = &PullSubscription{
						Spec: *spec,
					}
				}
			}
			updated := &PullSubscription{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
