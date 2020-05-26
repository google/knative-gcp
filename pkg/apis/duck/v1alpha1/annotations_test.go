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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

var (
	noScaling   = &v1.ObjectMeta{}
	kedaScaling = &v1.ObjectMeta{
		Annotations: map[string]string{
			AutoscalingClassAnnotation:                KEDA,
			AutoscalingMinScaleAnnotation:             "1",
			AutoscalingMaxScaleAnnotation:             "5",
			KedaAutoscalingPollingIntervalAnnotation:  "30",
			KedaAutoscalingCooldownPeriodAnnotation:   "60",
			KedaAutoscalingSubscriptionSizeAnnotation: "10",
		},
	}
)

func TestSetAutoscalingAnnotationsDefaults(t *testing.T) {
	testCases := map[string]struct {
		orig     *v1.ObjectMeta
		expected *v1.ObjectMeta
	}{
		"no defaults": {
			orig:     noScaling,
			expected: noScaling,
		},
		"minScale default": {
			orig: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				delete(obj.Annotations, AutoscalingMinScaleAnnotation)
				return obj
			}(),
			expected: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[AutoscalingMinScaleAnnotation] = defaultMinScale
				return obj
			}(),
		},
		"maxScale default": {
			orig: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				delete(obj.Annotations, AutoscalingMaxScaleAnnotation)
				return obj
			}(),
			expected: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[AutoscalingMaxScaleAnnotation] = defaultMaxScale
				return obj
			}(),
		},
		"pollingInterval default": {
			orig: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				delete(obj.Annotations, KedaAutoscalingPollingIntervalAnnotation)
				return obj
			}(),
			expected: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[KedaAutoscalingPollingIntervalAnnotation] = defaultKedaPollingInterval
				return obj
			}(),
		},
		"cooldownPeriod default": {
			orig: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				delete(obj.Annotations, KedaAutoscalingCooldownPeriodAnnotation)
				return obj
			}(),
			expected: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[KedaAutoscalingCooldownPeriodAnnotation] = defaultKedaCooldownPeriod
				return obj
			}(),
		},
		"subscriptionSize default": {
			orig: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				delete(obj.Annotations, KedaAutoscalingSubscriptionSizeAnnotation)
				return obj
			}(),
			expected: func() *v1.ObjectMeta {
				obj := kedaScaling.DeepCopy()
				obj.Annotations[KedaAutoscalingSubscriptionSizeAnnotation] = defaultKedaSubscriptionSize
				return obj
			}(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			SetAutoscalingAnnotationsDefaults(context.TODO(), tc.orig)
			if diff := cmp.Diff(tc.expected, tc.orig); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}

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

func TestSetClusterNameAnnotation(t *testing.T) {
	testCases := map[string]struct {
		orig     *v1.ObjectMeta
		data     testingMetadataClient.TestClientData
		expected *v1.ObjectMeta
	}{
		"no annotation, successfully get the clusterName": {
			orig: &v1.ObjectMeta{},
			data: testingMetadataClient.TestClientData{},
			expected: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
		},
		"no annotation, get clusterName failed": {
			orig: &v1.ObjectMeta{},
			data: testingMetadataClient.TestClientData{
				ClusterNameErr: fmt.Errorf("error when get clusterName"),
			},
			expected: &v1.ObjectMeta{},
		},
		"has annotation": {
			orig: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: "testing-cluster-name",
				},
			},
			data: testingMetadataClient.TestClientData{},
			expected: &v1.ObjectMeta{
				Annotations: map[string]string{
					ClusterNameAnnotation: "testing-cluster-name",
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			SetClusterNameAnnotation(tc.orig, testingMetadataClient.NewTestClient(tc.data))
			if diff := cmp.Diff(tc.expected, tc.orig); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
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
		"update empty annotation": {
			original: &v1.ObjectMeta{},
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
