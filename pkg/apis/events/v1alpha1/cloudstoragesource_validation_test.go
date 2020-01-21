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
					Ref: &corev1.ObjectReference{
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
					Ref: &corev1.ObjectReference{
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
					Ref: &corev1.ObjectReference{
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
						Ref: &corev1.ObjectReference{
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
						Ref: &corev1.ObjectReference{
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
						Ref: &corev1.ObjectReference{
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
						Ref: &corev1.ObjectReference{
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
						Ref: &corev1.ObjectReference{
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
