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

// Package duck contains Cloud Run Events API versions for duck components
package duck

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestValidateAutoscalingAnnotations(t *testing.T) {
	testCases := map[string]struct {
		objMeta *v1.ObjectMeta
		error   bool
	}{
		"ok no scaling": {
			objMeta: noScaling,
			error:   false,
		},
		"invalid extra resources": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				delete(obj.Annotations, AutoscalingClassAnnotation)
				return obj
			}(),
			error: true,
		},
		"ok keda scaling": {
			objMeta: kedaScaling,
			error:   false,
		},
		"unsupported scaling class": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[AutoscalingClassAnnotation] = "invalid"
				return obj
			}(),
			error: true,
		},
		"invalid min scale": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[AutoscalingMinScaleAnnotation] = "-1"
				return obj
			}(),
			error: true,
		},
		"invalid max scale": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[AutoscalingMaxScaleAnnotation] = "0"
				return obj
			}(),
			error: true,
		},
		"invalid min > max": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[AutoscalingMinScaleAnnotation] = "4"
				obj.Annotations[AutoscalingMaxScaleAnnotation] = "1"
				return obj
			}(),
			error: true,
		},
		"invalid pollingInterval": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[KedaAutoscalingPollingIntervalAnnotation] = "1"
				return obj
			}(),
			error: true,
		},
		"invalid cooldownPeriod": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[KedaAutoscalingCooldownPeriodAnnotation] = "0"
				return obj
			}(),
			error: true,
		},
		"invalid subscriptionSize": {
			objMeta: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[KedaAutoscalingSubscriptionSizeAnnotation] = "0"
				return obj
			}(),
			error: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var errs *apis.FieldError
			err := ValidateAutoscalingAnnotations(context.TODO(), tc.objMeta.Annotations, errs)
			if tc.error != (err != nil) {
				t.Fatalf("Unexpected validation failure. Got %v", err)
			}
		})
	}
}

func TestCheckImmutableClusterNameAnnotation(t *testing.T) {
	testCases := map[string]struct {
		original *v1.ObjectMeta
		current  *v1.ObjectMeta
		error    bool
	}{
		"unchanged nil annotation": {
			original: &v1.ObjectMeta{},
			current:  &v1.ObjectMeta{},
			error:    false,
		},
		"unchanged empty annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			error: false,
		},
		"update nil annotation": {
			original: &v1.ObjectMeta{},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			error: false,
		},
		"update empty annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			error: false,
		},
		"update non-empty annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: testingMetadataClient.FakeClusterName + "old",
				},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: testingMetadataClient.FakeClusterName + "new",
				},
			},
			error: true,
		},
		"unchanged annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: "testing-cluster-name",
				},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: "testing-cluster-name",
				},
			},
			error: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var err *apis.FieldError
			err = CheckImmutableClusterNameAnnotation(tc.current, tc.original, err)
			if tc.error != (err != nil) {
				t.Fatalf("Unexpected validation failure. Got %v", err)
			}
		})
	}
}

func TestCheckImmutableAutoscalingClassAnnotation(t *testing.T) {
	testCases := map[string]struct {
		original *v1.ObjectMeta
		current  *v1.ObjectMeta
		error    bool
	}{
		"unchanged nil annotation": {
			original: &v1.ObjectMeta{},
			current:  &v1.ObjectMeta{},
			error:    false,
		},
		"unchanged empty annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			error: false,
		},
		"update nil annotation": {
			original: &v1.ObjectMeta{},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: KEDA,
				},
			},
			error: true,
		},
		"update empty annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: KEDA,
				},
			},
			error: true,
		},
		"update non-empty annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: KEDA,
				},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: "new-value",
				},
			},
			error: true,
		},
		"delete annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: KEDA,
				},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{},
			},
			error: true,
		},
		"unchanged annotation": {
			original: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: KEDA,
				},
			},
			current: &v1.ObjectMeta{
				Annotations: map[string]string{
					AutoscalingClassAnnotation: KEDA,
				},
			},
			error: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var err *apis.FieldError
			err = CheckImmutableAutoscalingClassAnnotations(tc.current, tc.original, err)
			if tc.error != (err != nil) {
				t.Fatalf("Unexpected validation failure. Got %v", err)
			}
		})
	}
}

func TestValidateCredential(t *testing.T) {
	testCases := []struct {
		name           string
		secret         *corev1.SecretKeySelector
		serviceAccount string
		wantErr        bool
	}{{
		name:           "nil secret, and nil service account",
		secret:         nil,
		serviceAccount: "",
		wantErr:        false,
	}, {
		name:           "valid secret, and nil service account",
		secret:         &gcpauthtesthelper.Secret,
		serviceAccount: "",
		wantErr:        false,
	}, {
		name: "invalid secret, and nil service account",
		secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: gcpauthtesthelper.Secret.Name,
			},
		},
		serviceAccount: "",
		wantErr:        true,
	}, {
		name: "invalid secret, and nil service account",
		secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{},
			Key:                  gcpauthtesthelper.Secret.Key,
		},
		serviceAccount: "",
		wantErr:        true,
	}, {
		name:           "nil secret, and valid k8s service account",
		secret:         nil,
		serviceAccount: "test123",
		wantErr:        false,
	}, {
		name:           "nil secret, and invalid service account",
		secret:         nil,
		serviceAccount: "@test",
		wantErr:        true,
	}, {
		name:           "secret and service account exist at the same time",
		secret:         &gcpauthtesthelper.Secret,
		serviceAccount: "test",
		wantErr:        true,
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		errs := ValidateCredential(tc.secret, tc.serviceAccount)
		got := errs != nil
		if diff := cmp.Diff(tc.wantErr, got); diff != "" {
			t.Errorf("unexpected resource (-want, +got) = %v", diff)
		}
	}
}
