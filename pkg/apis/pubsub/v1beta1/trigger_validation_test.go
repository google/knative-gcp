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

package v1beta1

import (
	"context"
	"testing"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

var (
	sourcetype       = TriggerAuditLogs
	googleSourceSpec = TriggerSpec{
		Secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "secret-name",
			},
			Key: "secret-key",
		},
		Project:    "my-eventing-project",
		SourceType: sourcetype,
		Filters: map[string]string{
			"ServiceName":  "foo",
			"MethodName":   "bar",
			"ResourceName": "baz",
		},
	}
	validServiceAccountName   = "test123@test123.iam.gserviceaccount.com"
	invalidServiceAccountName = "test@test.iam.kserviceaccount.com"
)

func TestTriggerCheckValidationFields(t *testing.T) {
	testCases := map[string]struct {
		spec  TriggerSpec
		error bool
	}{
		"ok": {
			spec:  googleSourceSpec,
			error: false,
		},
		"no sourcetype": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
				obj.SourceType = ""
				return *obj
			}(),
			error: true,
		},
		"audit logs filters no serviceName": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
				delete(obj.Filters, "ServiceName")
				return *obj
			}(),
			error: true,
		},
		"audit logs filters no methodName": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
				delete(obj.Filters, "MethodName")
				return *obj
			}(),
			error: true,
		},
		"audit logs filters invalid filter key": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
				obj.Filters["invalid"] = "value"
				return *obj
			}(),
			error: true,
		},
		"invalid secret, missing key": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
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
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
				return *obj
			}(),
			error: false,
		},
		"invalid GCP service account": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
				obj.GoogleServiceAccount = invalidServiceAccountName
				return *obj
			}(),
			error: true,
		},
		"have GCP service account and secret at the same time": {
			spec: func() TriggerSpec {
				obj := googleSourceSpec.DeepCopy()
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

func TestTriggerCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated TriggerSpec
		allowed bool
	}{
		"nil orig": {
			updated: googleSourceSpec,
			allowed: true,
		},
		"Secret.Name changed": {
			orig: &googleSourceSpec,
			updated: TriggerSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "some-other-name",
					},
					Key: googleSourceSpec.Secret.Key,
				},
				Project:    googleSourceSpec.Project,
				SourceType: googleSourceSpec.SourceType,
				Filters:    googleSourceSpec.Filters,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &googleSourceSpec,
			updated: TriggerSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: googleSourceSpec.Secret.Name,
					},
					Key: "some-other-key",
				},
				Project:    googleSourceSpec.Project,
				SourceType: googleSourceSpec.SourceType,
				Filters:    googleSourceSpec.Filters,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &googleSourceSpec,
			updated: TriggerSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: googleSourceSpec.Secret.Name,
					},
					Key: googleSourceSpec.Secret.Key,
				},
				Project:    "some-other-project",
				SourceType: googleSourceSpec.SourceType,
				Filters:    googleSourceSpec.Filters,
			},
			allowed: false,
		},
		"ServiceAccount changed": {
			orig: &googleSourceSpec,
			updated: TriggerSpec{
				IdentitySpec: v1beta1.IdentitySpec{
					GoogleServiceAccount: "new-service-account",
				},
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: googleSourceSpec.Secret.Name,
					},
					Key: googleSourceSpec.Secret.Key,
				},
				SourceType: googleSourceSpec.SourceType,
				Filters:    googleSourceSpec.Filters,
			},
			allowed: false,
		},
		"SourceType changed": {
			orig: &googleSourceSpec,
			updated: TriggerSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: googleSourceSpec.Secret.Name,
					},
					Key: googleSourceSpec.Secret.Key,
				},
				Project:    googleSourceSpec.Project,
				SourceType: "test-google",
				Filters:    googleSourceSpec.Filters,
			},
			allowed: false,
		},
		"Filters changed": {
			orig: &googleSourceSpec,
			updated: TriggerSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: googleSourceSpec.Secret.Name,
					},
					Key: googleSourceSpec.Secret.Key,
				},
				Project:    googleSourceSpec.Project,
				SourceType: googleSourceSpec.SourceType,
				Filters: map[string]string{
					"ServiceName":  "changed",
					"MethodName":   "bar",
					"ResourceName": "baz",
				},
			},
			allowed: false,
		},
		"no change": {
			orig:    &googleSourceSpec,
			updated: googleSourceSpec,
			allowed: true,
		},
		"not spec": {
			orig:    []string{"wrong"},
			updated: googleSourceSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *Trigger

			if tc.orig != nil {
				if spec, ok := tc.orig.(*TriggerSpec); ok {
					orig = &Trigger{
						Spec: *spec,
					}
				}
			}
			updated := &Trigger{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
