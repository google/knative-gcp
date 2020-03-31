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
	"knative.dev/pkg/ptr"
)

var (
	pubSubSourceSpec = CloudPubSubSourceSpec{
		PubSubSpec: duckv1alpha1.PubSubSpec{
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret-name",
				},
				Key: "secret-key",
			},
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
			Project: "my-eventing-project",
		},
		Topic: "pubsub-topic",
	}
)

func TestCloudPubSubSourceCheckValidationFields(t *testing.T) {
	testCases := map[string]struct {
		spec  CloudPubSubSourceSpec
		error bool
	}{
		"ok": {
			spec:  pubSubSourceSpec,
			error: false,
		},
		"no topic": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.Topic = ""
				return *obj
			}(),
			error: true,
		},
		"bad RetentionDuration": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.RetentionDuration = ptr.String("wrong")
				return *obj
			}(),
			error: true,
		},
		"bad RetentionDuration, range": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.RetentionDuration = ptr.String("10000h")
				return *obj
			}(),
			error: true,
		},
		"bad AckDeadline": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.AckDeadline = ptr.String("wrong")
				return *obj
			}(),
			error: true,
		},
		"bad AckDeadline, range": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.AckDeadline = ptr.String("10000h")
				return *obj
			}(),
			error: true,
		},
		"bad sink, name": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.Sink.Ref.Name = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, apiVersion": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.Sink.Ref.APIVersion = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, kind": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.Sink.Ref.Kind = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, empty": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.Sink = duckv1.Destination{}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri scheme": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
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
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
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
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
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
		"invalid secret, missing key": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.Secret = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "name",
					},
				}
				return *obj
			}(),
			error: true,
		},
		"nil service account": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				return *obj
			}(),
			error: false,
		},
		"invalid GCP service account": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.GoogleServiceAccount = invalidServiceAccountName
				return *obj
			}(),
			error: true,
		},
		"have GCP service account and secret at the same time": {
			spec: func() CloudPubSubSourceSpec {
				obj := pubSubSourceSpec.DeepCopy()
				obj.GoogleServiceAccount = invalidServiceAccountName
				obj.Secret = duckv1alpha1.DefaultGoogleCloudSecretSelector()
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

func TestCloudPubSubSourceCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated CloudPubSubSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: pubSubSourceSpec,
			allowed: true,
		},
		"Secret.Name changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: pubSubSourceSpec.Sink,
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: "some-other-key",
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: pubSubSourceSpec.Sink,
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: "some-other-project",
					SourceSpec: duckv1.SourceSpec{
						Sink: pubSubSourceSpec.Sink,
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: false,
		},
		"ServiceAccount changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: "new-service-account",
					},
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					SourceSpec: duckv1.SourceSpec{
						Sink: pubSubSourceSpec.Sink,
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: false,
		},
		"Topic changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: pubSubSourceSpec.Sink,
					},
				},
				Topic: "some-other-topic",
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       pubSubSourceSpec.Sink.Ref.Kind,
								Namespace:  pubSubSourceSpec.Sink.Ref.Namespace,
								Name:       pubSubSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Kind changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: pubSubSourceSpec.Sink.Ref.APIVersion,
								Kind:       "some-other-kind",
								Namespace:  pubSubSourceSpec.Sink.Ref.Namespace,
								Name:       pubSubSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Namespace changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: pubSubSourceSpec.Sink.Ref.APIVersion,
								Kind:       pubSubSourceSpec.Sink.Ref.Kind,
								Namespace:  "some-other-namespace",
								Name:       pubSubSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Name changed": {
			orig: &pubSubSourceSpec,
			updated: CloudPubSubSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pubSubSourceSpec.Secret.Name,
						},
						Key: pubSubSourceSpec.Secret.Key,
					},
					Project: pubSubSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: pubSubSourceSpec.Sink.Ref.APIVersion,
								Kind:       pubSubSourceSpec.Sink.Ref.Kind,
								Namespace:  pubSubSourceSpec.Sink.Ref.Namespace,
								Name:       "some-other-name",
							},
						},
					},
				},
				Topic: pubSubSourceSpec.Topic,
			},
			allowed: true,
		},
		"no change": {
			orig:    &pubSubSourceSpec,
			updated: pubSubSourceSpec,
			allowed: true,
		},
		"not spec": {
			orig:    []string{"wrong"},
			updated: pubSubSourceSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *CloudPubSubSource

			if tc.orig != nil {
				if spec, ok := tc.orig.(*CloudPubSubSourceSpec); ok {
					orig = &CloudPubSubSource{
						Spec: *spec,
					}
				}
			}
			updated := &CloudPubSubSource{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
