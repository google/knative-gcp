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

package v1beta1

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"

	"github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
)

var (
	pullSubscriptionSpec = PullSubscriptionSpec{
		PubSubSpec: v1beta1.PubSubSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret-name",
				},
				Key: "secret-key",
			},
			Project: "my-eventing-project",
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "foo",
						Kind:       "bar",
						Namespace:  "baz",
						Name:       "qux",
					},
				},
			},
		},
		Topic: "pubsub-topic",
		Transformer: &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: "foo",
				Kind:       "bar",
				Namespace:  "baz",
				Name:       "qux",
			},
		},
		Mode: ModeCloudEventsStructured,
	}
)

func TestPubSubCheckValidationFields(t *testing.T) {
	testCases := map[string]struct {
		spec  PullSubscriptionSpec
		error bool
	}{
		"ok": {
			spec:  pullSubscriptionSpec,
			error: false,
		},
		"bad RetentionDuration": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.RetentionDuration = ptr.String("wrong")
				return *obj
			}(),
			error: true,
		},
		"bad RetentionDuration, range": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.RetentionDuration = ptr.String("10000h")
				return *obj
			}(),
			error: true,
		},
		"bad AckDeadline": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.AckDeadline = ptr.String("wrong")
				return *obj
			}(),
			error: true,
		},
		"bad AckDeadline, range": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.AckDeadline = ptr.String("10000h")
				return *obj
			}(),
			error: true,
		},
		"bad sink, name": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink.Ref.Name = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, apiVersion": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink.Ref.APIVersion = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, kind": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink.Ref.Kind = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, empty": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = duckv1.Destination{}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri scheme": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = duckv1.Destination{
					URI: &apis.URL{
						Host: "example.com",
					},
				}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri host": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = duckv1.Destination{
					URI: &apis.URL{
						Scheme: "http",
					},
				}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri and ref": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Sink = duckv1.Destination{
					URI: &apis.URL{
						Scheme: "http",
						Host:   "example.com",
					},
					Ref: &duckv1.KReference{
						Name: "foo",
					},
				}
				return *obj
			}(),
			error: true,
		},
		"bad transformer, name": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Transformer = obj.Sink.DeepCopy()
				obj.Transformer.Ref.Name = ""
				return *obj
			}(),
			error: true,
		},
		"bad secret, missing key": {
			spec: func() PullSubscriptionSpec {
				obj := pullSubscriptionSpec.DeepCopy()
				obj.Secret = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "some-other-name",
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
		updated PullSubscriptionSpec
		allowed bool
	}{
		"nil orig": {
			updated: pullSubscriptionSpec,
			allowed: true,
		},
		"Secret.Name changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
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
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
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
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
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
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: false,
		},
		"Topic changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
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
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       pullSubscriptionSpec.Sink.Ref.Kind,
								Namespace:  pullSubscriptionSpec.Sink.Ref.Namespace,
								Name:       pullSubscriptionSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Sink.Kind changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: pullSubscriptionSpec.Sink.Ref.APIVersion,
								Kind:       "some-other-kind",
								Namespace:  pullSubscriptionSpec.Sink.Ref.Namespace,
								Name:       pullSubscriptionSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Sink.Namespace changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: pullSubscriptionSpec.Sink.Ref.APIVersion,
								Kind:       pullSubscriptionSpec.Sink.Ref.Kind,
								Namespace:  "some-other-namespace",
								Name:       pullSubscriptionSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Sink.Name changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: pullSubscriptionSpec.Sink.Ref.APIVersion,
								Kind:       pullSubscriptionSpec.Sink.Ref.Kind,
								Namespace:  pullSubscriptionSpec.Sink.Ref.Namespace,
								Name:       "some-other-name",
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Transformer.APIVersion changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       pullSubscriptionSpec.Transformer.Ref.Kind,
								Namespace:  pullSubscriptionSpec.Transformer.Ref.Namespace,
								Name:       pullSubscriptionSpec.Transformer.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Mode:  pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Transformer.Kind changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       pullSubscriptionSpec.Transformer.Ref.Kind,
								Namespace:  pullSubscriptionSpec.Transformer.Ref.Namespace,
								Name:       pullSubscriptionSpec.Transformer.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Transformer: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: pullSubscriptionSpec.Transformer.Ref.APIVersion,
						Kind:       "some-other-kind",
						Namespace:  pullSubscriptionSpec.Transformer.Ref.Namespace,
						Name:       pullSubscriptionSpec.Transformer.Ref.Name,
					},
				},
				Mode: pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Transformer.Namespace changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       pullSubscriptionSpec.Transformer.Ref.Kind,
								Namespace:  pullSubscriptionSpec.Transformer.Ref.Namespace,
								Name:       pullSubscriptionSpec.Transformer.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Transformer: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: pullSubscriptionSpec.Transformer.Ref.APIVersion,
						Kind:       pullSubscriptionSpec.Transformer.Ref.Kind,
						Namespace:  "some-other-namespace",
						Name:       pullSubscriptionSpec.Transformer.Ref.Name,
					},
				},
				Mode: pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Transformer.Name changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pullSubscriptionSpec.Secret.Name,
						},
						Key: pullSubscriptionSpec.Secret.Key,
					},
					Project: pullSubscriptionSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       pullSubscriptionSpec.Transformer.Ref.Kind,
								Namespace:  pullSubscriptionSpec.Transformer.Ref.Namespace,
								Name:       pullSubscriptionSpec.Transformer.Ref.Name,
							},
						},
					},
				},
				Topic: pullSubscriptionSpec.Topic,
				Transformer: &duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: pullSubscriptionSpec.Transformer.Ref.APIVersion,
						Kind:       pullSubscriptionSpec.Transformer.Ref.Kind,
						Namespace:  pullSubscriptionSpec.Transformer.Ref.Namespace,
						Name:       "some-other-name",
					},
				},
				Mode: pullSubscriptionSpec.Mode,
			},
			allowed: true,
		},
		"Mode changed": {
			orig: &pullSubscriptionSpec,
			updated: PullSubscriptionSpec{
				PubSubSpec: v1beta1.PubSubSpec{
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
				Topic: pullSubscriptionSpec.Topic,
				Mode:  ModePushCompatible,
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
