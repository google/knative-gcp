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

	cloudevents "github.com/cloudevents/sdk-go"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

var (
	// Bare minimum is Bucket and Sink
	minimalCloudStorageSourceSpec = CloudStorageSourceSpec{
		Bucket: "my-test-bucket",
		PubSubSpec: duckv1alpha1.PubSubSpec{
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
	}

	// Bucket, Sink and Secret
	withSecret = CloudStorageSourceSpec{
		Bucket: "my-test-bucket",
		PubSubSpec: duckv1alpha1.PubSubSpec{
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
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret-name",
				},
				Key: "secret-key",
			},
		},
	}

	// Bucket, Sink, Secret, and PubSubSecret
	withPubSubSecret = CloudStorageSourceSpec{
		Bucket: "my-test-bucket",
		PubSubSpec: duckv1alpha1.PubSubSpec{
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
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "gcs-secret-name",
				},
				Key: "gcs-secret-key",
			},
			PubSubSecret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "pullsubscription-secret-name",
				},
				Key: "pullsubscription-secret-key",
			},
		},
	}
	// Bucket, Sink, Secret, PubSubSecret, Event Type and Project, ObjectNamePrefix and PayloadFormat
	storageSourceSpec = CloudStorageSourceSpec{
		Bucket:           "my-test-bucket",
		EventTypes:       []string{CloudStorageSourceFinalize, CloudStorageSourceDelete},
		ObjectNamePrefix: "test-prefix",
		PayloadFormat:    cloudevents.ApplicationJSON,
		PubSubSpec: duckv1alpha1.PubSubSpec{
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
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "gcs-secret-name",
				},
				Key: "gcs-secret-key",
			},
			Project: "my-eventing-project",
			PubSubSecret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "pullsubscription-secret-name",
				},
				Key: "pullsubscription-secret-key",
			},
		},
	}
)

func TestValidationFields(t *testing.T) {
	testCases := []struct {
		name string
		s    *CloudStorageSource
		want *apis.FieldError
	}{{
		name: "empty",
		s:    &CloudStorageSource{Spec: CloudStorageSourceSpec{}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.bucket", "spec.sink")
			return fe
		}(),
	}, {
		name: "missing sink",
		s:    &CloudStorageSource{Spec: CloudStorageSourceSpec{Bucket: "foo"}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.sink")
			return fe
		}(),
	}}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate CloudStorageSourceSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSpecValidationFields(t *testing.T) {
	testCases := []struct {
		name string
		spec *CloudStorageSourceSpec
		want *apis.FieldError
	}{{
		name: "empty",
		spec: &CloudStorageSourceSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("bucket", "sink")
			return fe
		}(),
	}, {
		name: "missing sink",
		spec: &CloudStorageSourceSpec{Bucket: "foo"},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("sink")
			return fe
		}(),
	}, {
		name: "missing bucket",
		spec: &CloudStorageSourceSpec{
			PubSubSpec: duckv1alpha1.PubSubSpec{
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
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("bucket")
			return fe
		}(),
	}, {
		name: "invalid secret, missing name",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1alpha1.PubSubSpec{
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
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{},
					Key:                  "secret-test-key",
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("secret.name")
			return fe
		}(),
	}, {
		name: "invalid gcs secret, missing key",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1alpha1.PubSubSpec{
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
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "gcs-test-secret"},
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("secret.key")
			return fe
		}(),
	}, {
		name: "invalid pullsubscription secret, missing name",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1alpha1.PubSubSpec{
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
				PubSubSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{},
					Key:                  "secret-test-key",
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("pubSubSecret.name")
			return fe
		}(),
	}, {
		name: "invalid gcs secret, missing key",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1alpha1.PubSubSpec{
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
				PubSubSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "gcs-test-secret"},
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("pubSubSecret.key")
			return fe
		}(),
	}}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate CloudStorageSourceSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated CloudStorageSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: storageSourceSpec,
			allowed: true,
		},
		"Bucket changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           "some-other-bucket",
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    storageSourceSpec.PayloadFormat,
				PubSubSpec:       storageSourceSpec.PubSubSpec,
			},
			allowed: false,
		},
		"EventType changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       []string{CloudStorageSourceMetadataUpdate},
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    storageSourceSpec.PayloadFormat,
				PubSubSpec:       storageSourceSpec.PubSubSpec,
			},
			allowed: false,
		},
		"ObjectNamePrefix changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: "some-other-prefix",
				PayloadFormat:    storageSourceSpec.PayloadFormat,
				PubSubSpec:       storageSourceSpec.PubSubSpec,
			},
			allowed: false,
		},
		"PayloadFormat changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    "some-other-format",
				PubSubSpec:       storageSourceSpec.PubSubSpec,
			},
			allowed: false,
		},
		"Secret.Name changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    storageSourceSpec.PayloadFormat,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					SourceSpec: storageSourceSpec.SourceSpec,
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: storageSourceSpec.Secret.Key,
					},
					Project:      storageSourceSpec.Project,
					PubSubSecret: storageSourceSpec.PubSubSecret,
				},
			},
			allowed: false,
		},
		"PubSubSecret.Name changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    storageSourceSpec.PayloadFormat,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					SourceSpec: storageSourceSpec.SourceSpec,
					Secret:     storageSourceSpec.Secret,
					Project:    storageSourceSpec.Project,
					PubSubSecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-pullsubscription-secret-name",
						},
						Key: storageSourceSpec.PubSubSecret.Key,
					},
				},
			},
			allowed: false,
		},
		"Project changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    storageSourceSpec.PayloadFormat,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					SourceSpec:   storageSourceSpec.SourceSpec,
					Secret:       storageSourceSpec.Secret,
					Project:      "some-other-project",
					PubSubSecret: storageSourceSpec.PubSubSecret,
				},
			},
			allowed: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *CloudStorageSource

			if tc.orig != nil {
				if spec, ok := tc.orig.(*CloudStorageSourceSpec); ok {
					orig = &CloudStorageSource{
						Spec: *spec,
					}
				}
			}
			updated := &CloudStorageSource{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
