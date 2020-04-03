/*
Copyright 2020 Google LLC

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
	topic           = DefaultTopic
	buildSourceSpec = CloudBuildSourceSpec{
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
		Topic: ptr.String(topic),
	}
)

func TestCloudBuildSourceCheckValidationFields(t *testing.T) {
	testCases := map[string]struct {
		spec  CloudBuildSourceSpec
		error bool
	}{
		"ok": {
			spec:  buildSourceSpec,
			error: false,
		},
		"no topic": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.Topic = nil
				return *obj
			}(),
			error: false,
		},
		"bad topic": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.Topic = ptr.String("test-build")
				return *obj
			}(),
			error: true,
		},
		"bad sink, name": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.Sink.Ref.Name = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, apiVersion": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.Sink.Ref.APIVersion = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, kind": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.Sink.Ref.Kind = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, empty": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.Sink = duckv1.Destination{}
				return *obj
			}(),
			error: true,
		},
		"bad sink, uri scheme": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
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
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
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
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
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
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
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
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				return *obj
			}(),
			error: false,
		},
		"invalid GCP service account": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
				obj.GoogleServiceAccount = invalidServiceAccountName
				return *obj
			}(),
			error: true,
		},
		"have GCP service account and secret at the same time": {
			spec: func() CloudBuildSourceSpec {
				obj := buildSourceSpec.DeepCopy()
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

func TestCloudBuildSourceCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated CloudBuildSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: buildSourceSpec,
			allowed: true,
		},
		"Secret.Name changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: buildSourceSpec.Sink,
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: "some-other-key",
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: buildSourceSpec.Sink,
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: "some-other-project",
					SourceSpec: duckv1.SourceSpec{
						Sink: buildSourceSpec.Sink,
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: false,
		},
		"ServiceAccount changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: "new-service-account",
					},
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					SourceSpec: duckv1.SourceSpec{
						Sink: buildSourceSpec.Sink,
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: false,
		},
		"Topic changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: buildSourceSpec.Sink,
					},
				},
				Topic: ptr.String("test-build"),
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       buildSourceSpec.Sink.Ref.Kind,
								Namespace:  buildSourceSpec.Sink.Ref.Namespace,
								Name:       buildSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Kind changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: buildSourceSpec.Sink.Ref.APIVersion,
								Kind:       "some-other-kind",
								Namespace:  buildSourceSpec.Sink.Ref.Namespace,
								Name:       buildSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Namespace changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: buildSourceSpec.Sink.Ref.APIVersion,
								Kind:       buildSourceSpec.Sink.Ref.Kind,
								Namespace:  "some-other-namespace",
								Name:       buildSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: true,
		},
		"Sink.Name changed": {
			orig: &buildSourceSpec,
			updated: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildSourceSpec.Secret.Name,
						},
						Key: buildSourceSpec.Secret.Key,
					},
					Project: buildSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: buildSourceSpec.Sink.Ref.APIVersion,
								Kind:       buildSourceSpec.Sink.Ref.Kind,
								Namespace:  buildSourceSpec.Sink.Ref.Namespace,
								Name:       "some-other-name",
							},
						},
					},
				},
				Topic: buildSourceSpec.Topic,
			},
			allowed: true,
		},
		"no change": {
			orig:    &buildSourceSpec,
			updated: buildSourceSpec,
			allowed: true,
		},
		"not spec": {
			orig:    []string{"wrong"},
			updated: buildSourceSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *CloudBuildSource

			if tc.orig != nil {
				if spec, ok := tc.orig.(*CloudBuildSourceSpec); ok {
					orig = &CloudBuildSource{
						Spec: *spec,
					}
				}
			}
			updated := &CloudBuildSource{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
