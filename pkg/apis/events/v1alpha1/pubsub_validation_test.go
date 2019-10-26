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

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/apis/v1alpha1"
	"knative.dev/pkg/ptr"
)

var (
	pullSubscriptionSpec = PubSubSpec{
		PubSubSpec: duckv1alpha1.PubSubSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret-name",
				},
				Key: "secret-key",
			},
			SourceSpec: duckv1.SourceSpec{
				Sink: v1alpha1.Destination{
					Ref: &corev1.ObjectReference{
						APIVersion: "foo",
						Kind:       "bar",
						Namespace:  "baz",
						Name:       "qux",
					},
				},
			},
			Project: "my-eventing-project",
		},
		Topic: "pubsub-topic",
	}
)

func TestPubSubCheckValidationFields(t *testing.T) {
	testCases := map[string]struct {
		spec  PubSubSpec
		error bool
	}{
		"ok": {
			spec:  pullSubscriptionSpec,
			error: false,
		},
		"bad RetentionDuration": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.RetentionDuration = ptr.String("wrong")
				return *obj
			}(),
			error: true,
		},
		"bad RetentionDuration, range": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.RetentionDuration = ptr.String("10000h")
				return *obj
			}(),
			error: true,
		},
		"bad AckDeadline": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.AckDeadline = ptr.String("wrong")
				return *obj
			}(),
			error: true,
		},
		"bad AckDeadline, range": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.AckDeadline = ptr.String("10000h")
				return *obj
			}(),
			error: true,
		},
		"bad sink, name": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink.Ref.Name = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, apiVersion": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink.Ref.APIVersion = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, kind": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink.Ref.Kind = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, empty": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = v1alpha1.Destination{}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri scheme": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = v1alpha1.Destination{
					URI: &apis.URL{
						Host: "example.com",
					},
				}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri host": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = v1alpha1.Destination{
					URI: &apis.URL{
						Scheme: "http",
					},
				}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri and ref": {
			spec: func() PubSubSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = v1alpha1.Destination{
					URI: &apis.URL{
						Scheme: "http",
						Host:   "example.com",
					},
					Ref: &corev1.ObjectReference{
						Name: "foo",
					},
				}
				return *obj
			}(),
			error: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			err := tc.spec.Validate(context.TODO())
			if tc.error != (err != nil) {
				t.Fatalf("Unexpected validation failure. Got %v", err)
			}
		})
	}
}

func TestPubSubCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated PubSubSpec
		allowed bool
	}{
		"nil orig": {
			updated: pullSubscriptionSpec,
			allowed: true,
		},
		"Secret.Name changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: pullSubscriptionSpec.Sink,
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: "some-other-key",
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: pullSubscriptionSpec.Sink,
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: "some-other-project",
					SourceSpec: duckv1.SourceSpec{
						Sink: pullSubscriptionSpec.Sink,
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: false,
		},
		"Topic changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: pullSubscriptionSpec.Sink,
					},
				},
				Topic: "some-other-topic",
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: v1alpha1.Destination{
							Ref: &corev1.ObjectReference{
								APIVersion: "some-other-api-version",
								Kind:       pullSubscriptionSpec.Sink.DeprecatedKind,
								Namespace:  pullSubscriptionSpec.Sink.DeprecatedNamespace,
								Name:       pullSubscriptionSpec.Sink.DeprecatedName,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Kind changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: v1alpha1.Destination{
							Ref: &corev1.ObjectReference{
								APIVersion: pullSubscriptionSpec.Sink.DeprecatedAPIVersion,
								Kind:       "some-other-kind",
								Namespace:  pullSubscriptionSpec.Sink.DeprecatedNamespace,
								Name:       pullSubscriptionSpec.Sink.DeprecatedName,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Namespace changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: v1alpha1.Destination{
							Ref: &corev1.ObjectReference{
								APIVersion: pullSubscriptionSpec.Sink.DeprecatedAPIVersion,
								Kind:       pullSubscriptionSpec.Sink.DeprecatedKind,
								Namespace:  "some-other-namespace",
								Name:       pullSubscriptionSpec.Sink.DeprecatedName,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Name changed": {
			orig: &pullSubscriptionSpec,
			updated: PubSubSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: v1alpha1.Destination{
							Ref: &corev1.ObjectReference{
								APIVersion: pullSubscriptionSpec.Sink.DeprecatedAPIVersion,
								Kind:       pullSubscriptionSpec.Sink.DeprecatedKind,
								Namespace:  pullSubscriptionSpec.Sink.DeprecatedNamespace,
								Name:       "some-other-name",
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
			},
			allowed: true,
		},
		"no change": {
			orig:    &pullSubscriptionSpec,
			updated: pullSubscriptionSpec,
			allowed: true,
		},
		"not spec": {
			orig:    []string{"wrong"},
			updated: pullSubscriptionSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *PubSub

			if tc.orig != nil {
				if spec, ok := tc.orig.(*PubSubSpec); ok {
					orig = &PubSub{
						Spec: *spec,
					}
				}
			}
			updated := &PubSub{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
