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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/apis/duck"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	metadatatesting "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

var (
	// Bare minimum is Bucket and Sink
	minimalCloudStorageSourceSpec = CloudStorageSourceSpec{
		Bucket: "my-test-bucket",
		PubSubSpec: duckv1beta1.PubSubSpec{
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
		PubSubSpec: duckv1beta1.PubSubSpec{
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

	storageSourceSpecWithKSA = CloudStorageSourceSpec{
		Bucket: "my-test-bucket",
		PubSubSpec: duckv1beta1.PubSubSpec{
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
			IdentitySpec: duckv1beta1.IdentitySpec{
				ServiceAccountName: "old-service-account",
			},
		},
	}

	// Bucket, Sink, Secret, Event Type and Project, ObjectNamePrefix and PayloadFormat
	storageSourceSpec = CloudStorageSourceSpec{
		Bucket:           "my-test-bucket",
		EventTypes:       []string{schemasv1.CloudStorageObjectFinalizedEventType, schemasv1.CloudStorageObjectDeletedEventType},
		ObjectNamePrefix: "test-prefix",
		PayloadFormat:    cloudevents.ApplicationJSON,
		PubSubSpec: duckv1beta1.PubSubSpec{
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
		name: "invalid sink",
		spec: &CloudStorageSourceSpec{Bucket: "foo",
			PubSubSpec: duckv1beta1.PubSubSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "foo",
							Namespace:  "baz",
							Name:       "qux",
						},
					},
				}}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("sink.ref.kind")
			return fe
		}(),
	}, {
		name: "missing bucket",
		spec: &CloudStorageSourceSpec{
			PubSubSpec: duckv1beta1.PubSubSpec{
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
			PubSubSpec: duckv1beta1.PubSubSpec{
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
		name: "nil secret",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1beta1.PubSubSpec{
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
		want: nil,
	}, {
		name: "invalid gcs secret, missing key",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1beta1.PubSubSpec{
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
		name: "invalid k8s service account",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1beta1.PubSubSpec{
				IdentitySpec: duckv1beta1.IdentitySpec{
					ServiceAccountName: invalidServiceAccountName,
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
			},
		},
		want: func() *apis.FieldError {
			fe := &apis.FieldError{
				Message: `invalid value: @test, serviceAccountName should have format: ^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`,
				Paths:   []string{"serviceAccountName"},
			}
			return fe
		}(),
	}, {
		name: "valid k8s service account",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1beta1.PubSubSpec{
				IdentitySpec: duckv1beta1.IdentitySpec{
					ServiceAccountName: validServiceAccountName,
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
			},
		},
		want: nil,
	}, {
		name: "have k8s service account and secret at the same time",
		spec: &CloudStorageSourceSpec{
			Bucket: "my-test-bucket",
			PubSubSpec: duckv1beta1.PubSubSpec{
				IdentitySpec: duckv1beta1.IdentitySpec{
					ServiceAccountName: validServiceAccountName,
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
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{},
					Key:                  "secret-test-key",
				},
			},
		},
		want: func() *apis.FieldError {
			fe := &apis.FieldError{
				Message: "Can't have spec.serviceAccountName and spec.secret at the same time",
				Paths:   []string{""},
			}
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
		orig              interface{}
		updated           CloudStorageSourceSpec
		origAnnotation    map[string]string
		updatedAnnotation map[string]string
		allowed           bool
	}{
		"nil orig": {
			updated: storageSourceSpec,
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
				EventTypes:       []string{schemasv1.CloudStorageObjectMetadataUpdatedEventType},
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
				PubSubSpec: duckv1beta1.PubSubSpec{
					SourceSpec: storageSourceSpec.SourceSpec,
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: storageSourceSpec.Secret.Key,
					},
					Project: storageSourceSpec.Project,
				},
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PubSubSpec: duckv1beta1.PubSubSpec{
					SourceSpec: storageSourceSpec.SourceSpec,
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: storageSourceSpec.Secret.Name,
						},
						Key: "some-other-key",
					},
					Project: storageSourceSpec.Project,
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
				PubSubSpec: duckv1beta1.PubSubSpec{
					SourceSpec: storageSourceSpec.SourceSpec,
					Secret:     storageSourceSpec.Secret,
					Project:    "some-other-project",
				},
			},
			allowed: false,
		},
		"ServiceAccountName changed": {
			orig: &storageSourceSpecWithKSA,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpecWithKSA.Bucket,
				EventTypes:       storageSourceSpecWithKSA.EventTypes,
				ObjectNamePrefix: storageSourceSpecWithKSA.ObjectNamePrefix,
				PayloadFormat:    storageSourceSpecWithKSA.PayloadFormat,
				PubSubSpec: duckv1beta1.PubSubSpec{
					IdentitySpec: duckv1beta1.IdentitySpec{
						ServiceAccountName: "new-service-account",
					},
					SourceSpec: storageSourceSpecWithKSA.SourceSpec,
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
		"Sink.APIVersion changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    cloudevents.ApplicationJSON,
				PubSubSpec: duckv1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: storageSourceSpec.Secret.Name,
						},
						Key: storageSourceSpec.Secret.Key,
					},
					Project: storageSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "some-other-api-version",
								Kind:       storageSourceSpec.Sink.Ref.Kind,
								Namespace:  storageSourceSpec.Sink.Ref.Namespace,
								Name:       storageSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
			},
			allowed: true,
		},
		"Sink.Kind changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    cloudevents.ApplicationJSON,
				PubSubSpec: duckv1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: storageSourceSpec.Secret.Name,
						},
						Key: storageSourceSpec.Secret.Key,
					},
					Project: storageSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: storageSourceSpec.Sink.Ref.APIVersion,
								Kind:       "some-other-kind",
								Namespace:  storageSourceSpec.Sink.Ref.Namespace,
								Name:       storageSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
			},
			allowed: true,
		},
		"Sink.Namespace changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    cloudevents.ApplicationJSON,
				PubSubSpec: duckv1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: storageSourceSpec.Secret.Name,
						},
						Key: storageSourceSpec.Secret.Key,
					},
					Project: storageSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: storageSourceSpec.Sink.Ref.APIVersion,
								Kind:       storageSourceSpec.Sink.Ref.Kind,
								Namespace:  "some-other-namespace",
								Name:       storageSourceSpec.Sink.Ref.Name,
							},
						},
					},
				},
			},
			allowed: true,
		},
		"Sink.Name changed": {
			orig: &storageSourceSpec,
			updated: CloudStorageSourceSpec{
				Bucket:           storageSourceSpec.Bucket,
				EventTypes:       storageSourceSpec.EventTypes,
				ObjectNamePrefix: storageSourceSpec.ObjectNamePrefix,
				PayloadFormat:    cloudevents.ApplicationJSON,
				PubSubSpec: duckv1beta1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: storageSourceSpec.Secret.Name,
						},
						Key: storageSourceSpec.Secret.Key,
					},
					Project: storageSourceSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: storageSourceSpec.Sink.Ref.APIVersion,
								Kind:       storageSourceSpec.Sink.Ref.Kind,
								Namespace:  storageSourceSpec.Sink.Ref.Namespace,
								Name:       "some-other-name",
							},
						},
					},
				},
			},
			allowed: true,
		},
		"no change": {
			orig:    &storageSourceSpec,
			updated: storageSourceSpec,
			allowed: true,
		},
		"no spec": {
			orig:    []string{"wrong"},
			updated: storageSourceSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *CloudStorageSource

			if tc.origAnnotation != nil {
				orig = &CloudStorageSource{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tc.origAnnotation,
					},
				}
			} else if tc.orig != nil {
				if spec, ok := tc.orig.(*CloudStorageSourceSpec); ok {
					orig = &CloudStorageSource{
						Spec: *spec,
					}
				}
			}
			updated := &CloudStorageSource{
				ObjectMeta: v1.ObjectMeta{
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
