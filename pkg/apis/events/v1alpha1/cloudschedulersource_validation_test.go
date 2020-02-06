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

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	// Bare minimum is Location, Schedule, Data, and Sink
	minimalCloudSchedulerSourceSpec = CloudSchedulerSourceSpec{
		Location: "mylocation",
		Schedule: "* * * * *",
		Data:     "mydata",
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

	// Location, Schedule, Data, Sink and Secret
	schedulerWithSecret = CloudSchedulerSourceSpec{
		Location: "mylocation",
		Schedule: "* * * * *",
		Data:     "mydata",
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

	// Location, Schedule, Data, Sink, Secret, and PubSubSecret
	schedulerWithPubSubSecret = CloudSchedulerSourceSpec{
		Location: "mylocation",
		Schedule: "* * * * *",
		Data:     "mydata",
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
)

func TestCloudSchedulerSourceValidationFields(t *testing.T) {
	testCases := []struct {
		name string
		s    *CloudSchedulerSource
		want *apis.FieldError
	}{{
		name: "empty",
		s:    &CloudSchedulerSource{Spec: CloudSchedulerSourceSpec{}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.location", "spec.data", "spec.schedule", "spec.sink")
			return fe
		}(),
	}, {
		name: "missing data, schedule, and sink",
		s:    &CloudSchedulerSource{Spec: CloudSchedulerSourceSpec{Location: "location"}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.data", "spec.schedule", "spec.sink")
			return fe
		}(),
	}, {
		name: "missing schedule, and sink",
		s:    &CloudSchedulerSource{Spec: CloudSchedulerSourceSpec{Location: "location", Data: "data"}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.schedule", "spec.sink")
			return fe
		}(),
	}, {
		name: "missing sink",
		s:    &CloudSchedulerSource{Spec: CloudSchedulerSourceSpec{Location: "location", Data: "data", Schedule: "* * * * *"}},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.sink")
			return fe
		}(),
	}}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate CloudSchedulerSourceSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestCloudSchedulerSourceSpecValidationFields(t *testing.T) {
	testCases := []struct {
		name string
		spec *CloudSchedulerSourceSpec
		want *apis.FieldError
	}{{
		name: "empty",
		spec: &CloudSchedulerSourceSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("data", "location", "schedule", "sink")
			return fe
		}(),
	}, {
		name: "missing data, schedule, and sink",
		spec: &CloudSchedulerSourceSpec{Location: "location"},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("data", "schedule", "sink")
			return fe
		}(),
	}, {
		name: "missing schedule and data",
		spec: &CloudSchedulerSourceSpec{
			Location: "location",
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
			fe := apis.ErrMissingField("data", "schedule")
			return fe
		}(),
	}, {
		name: "missing data",
		spec: &CloudSchedulerSourceSpec{
			Location: "location",
			Schedule: "* * * * *",
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
			fe := apis.ErrMissingField("data")
			return fe
		}(),
	}, {
		name: "invalid secret, missing name",
		spec: &CloudSchedulerSourceSpec{
			Location: "my-test-location",
			Schedule: "* * * * *",
			Data:     "data",
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
		spec: &CloudSchedulerSourceSpec{
			Location: "my-test-location",
			Schedule: "* * * * *",
			Data:     "data",
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
		name: "invalid pubsub secret, missing name",
		spec: &CloudSchedulerSourceSpec{
			Location: "my-test-location",
			Schedule: "* * * * *",
			Data:     "data",
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
			fe := apis.ErrMissingField("pubsubSecret.name")
			return fe
		}(),
	}, {
		name: "invalid secret, missing key",
		spec: &CloudSchedulerSourceSpec{
			Location: "my-test-location",
			Schedule: "* * * * *",
			Data:     "data",
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
			fe := apis.ErrMissingField("pubsubSecret.key")
			return fe
		}(),
	}}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate CloudSchedulerSourceSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}

}

func TestCloudSchedulerSourceSpecCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated CloudSchedulerSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: schedulerWithPubSubSecret,
			allowed: true,
		},
		"Location changed": {
			orig: &schedulerWithPubSubSecret,
			updated: CloudSchedulerSourceSpec{
				Location:   "some-other-location",
				Schedule:   schedulerWithPubSubSecret.Schedule,
				Data:       schedulerWithPubSubSecret.Data,
				PubSubSpec: schedulerWithPubSubSecret.PubSubSpec,
			},
			allowed: false,
		},
		"Schedule changed": {
			orig: &schedulerWithPubSubSecret,
			updated: CloudSchedulerSourceSpec{
				Location:   schedulerWithPubSubSecret.Location,
				Schedule:   "* * * * 1",
				Data:       schedulerWithPubSubSecret.Data,
				PubSubSpec: schedulerWithPubSubSecret.PubSubSpec,
			},
			allowed: false,
		},
		"Data changed": {
			orig: &schedulerWithPubSubSecret,
			updated: CloudSchedulerSourceSpec{
				Location:   schedulerWithPubSubSecret.Location,
				Schedule:   schedulerWithPubSubSecret.Schedule,
				Data:       "some-other-data",
				PubSubSpec: schedulerWithPubSubSecret.PubSubSpec,
			},
			allowed: false,
		},
		"Secret.Name changed": {
			orig: &schedulerWithPubSubSecret,
			updated: CloudSchedulerSourceSpec{
				Location: schedulerWithPubSubSecret.Location,
				Schedule: schedulerWithPubSubSecret.Schedule,
				Data:     schedulerWithPubSubSecret.Data,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					SourceSpec: schedulerWithPubSubSecret.SourceSpec,
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: schedulerWithPubSubSecret.Secret.Key,
					},
					Project:      schedulerWithPubSubSecret.Project,
					PubSubSecret: schedulerWithPubSubSecret.PubSubSecret,
				},
			},
			allowed: false,
		},
		"PubSubSecret.Name changed": {
			orig: &schedulerWithPubSubSecret,
			updated: CloudSchedulerSourceSpec{
				Location: schedulerWithPubSubSecret.Location,
				Schedule: schedulerWithPubSubSecret.Schedule,
				Data:     schedulerWithPubSubSecret.Data,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					SourceSpec: schedulerWithPubSubSecret.SourceSpec,
					Secret:     schedulerWithPubSubSecret.Secret,
					Project:    schedulerWithPubSubSecret.Project,
					PubSubSecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-pullsubscription-secret-name",
						},
						Key: schedulerWithPubSubSecret.PubSubSecret.Key,
					},
				},
			},
			allowed: false,
		},
		"Project changed changed": {
			orig: &schedulerWithPubSubSecret,
			updated: CloudSchedulerSourceSpec{
				Location: schedulerWithPubSubSecret.Location,
				Schedule: schedulerWithPubSubSecret.Schedule,
				Data:     schedulerWithPubSubSecret.Data,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					SourceSpec:   schedulerWithPubSubSecret.SourceSpec,
					Secret:       schedulerWithPubSubSecret.Secret,
					Project:      "some-other-project",
					PubSubSecret: schedulerWithPubSubSecret.PubSubSecret,
				},
			},
			allowed: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *CloudSchedulerSource

			if tc.orig != nil {
				if spec, ok := tc.orig.(*CloudSchedulerSourceSpec); ok {
					orig = &CloudSchedulerSource{
						Spec: *spec,
					}
				}
			}
			updated := &CloudSchedulerSource{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
