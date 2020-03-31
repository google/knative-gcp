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
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	auditLogsSourceSpec = CloudAuditLogsSourceSpec{
		ServiceName:  "foo",
		MethodName:   "bar",
		ResourceName: "baz",
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
	}
	validServiceAccountName   = "test123@test123.iam.gserviceaccount.com"
	invalidServiceAccountName = "test@test.iam.kserviceaccount.com"
)

func TestCloudAuditLogsSourceValidationFields(t *testing.T) {
	testCases := map[string]struct {
		spec  CloudAuditLogsSourceSpec
		error bool
	}{
		"ok": {
			spec:  auditLogsSourceSpec,
			error: false,
		},
		"bad ServiceName": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				obj.ServiceName = ""
				return *obj
			}(),
			error: true,
		},
		"bad  MethodName": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				obj.MethodName = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, name": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				obj.Sink.Ref.Name = ""
				return *obj
			}(),
			error: true,
		},
		"bad sink, empty": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				obj.Sink = duckv1.Destination{}
				return *obj
			}(),
			error: true,
		},
		"nil secret": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				return *obj
			}(),
			error: false,
		},
		"invalid scheduler secret, missing key": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				obj.Secret = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
				}
				return *obj
			}(),
			error: true,
		},
		"invalid GCP service account": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
				obj.GoogleServiceAccount = invalidServiceAccountName
				return *obj
			}(),
			error: true,
		},
		"have GCP service account and secret at the same time": {
			spec: func() CloudAuditLogsSourceSpec {
				obj := auditLogsSourceSpec.DeepCopy()
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

func TestCloudAuditLogsSourceCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated CloudAuditLogsSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: auditLogsSourceSpec,
			allowed: true,
		},
		"ServiceName changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				MethodName:   auditLogsSourceSpec.MethodName,
				PubSubSpec:   auditLogsSourceSpec.PubSubSpec,
				ResourceName: auditLogsSourceSpec.ResourceName,
				ServiceName:  "some-other-name",
			},
			allowed: false,
		},
		"MethodName changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				MethodName:   "some-other-name",
				PubSubSpec:   auditLogsSourceSpec.PubSubSpec,
				ResourceName: auditLogsSourceSpec.ResourceName,
				ServiceName:  auditLogsSourceSpec.ServiceName,
			},
			allowed: false,
		},
		"ResourceName changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				MethodName:   auditLogsSourceSpec.MethodName,
				PubSubSpec:   auditLogsSourceSpec.PubSubSpec,
				ResourceName: "some-other-name",
				ServiceName:  auditLogsSourceSpec.ServiceName,
			},
			allowed: false,
		},
		"Project changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: auditLogsSourceSpec.PubSubSpec.Secret.Name,
						},
						Key: auditLogsSourceSpec.PubSubSpec.Secret.Key,
					},
					Project: "some-other-project",
					SourceSpec: duckv1.SourceSpec{
						Sink: auditLogsSourceSpec.PubSubSpec.Sink,
					},
				},
				MethodName:   auditLogsSourceSpec.MethodName,
				ResourceName: auditLogsSourceSpec.ResourceName,
				ServiceName:  auditLogsSourceSpec.ServiceName,
			},
			allowed: false,
		},
		"ServiceAccount changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: "new-service-account",
					},
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: auditLogsSourceSpec.PubSubSpec.Secret.Name,
						},
						Key: auditLogsSourceSpec.PubSubSpec.Secret.Key,
					},
					SourceSpec: duckv1.SourceSpec{
						Sink: auditLogsSourceSpec.PubSubSpec.Sink,
					},
				},
				MethodName:   auditLogsSourceSpec.MethodName,
				ResourceName: auditLogsSourceSpec.ResourceName,
				ServiceName:  auditLogsSourceSpec.ServiceName,
			},
			allowed: false,
		},
		"Secret.Name changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "some-other-name",
						},
						Key: auditLogsSourceSpec.PubSubSpec.Secret.Key,
					},
					Project: auditLogsSourceSpec.PubSubSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: auditLogsSourceSpec.PubSubSpec.Sink,
					},
				},
				MethodName:   auditLogsSourceSpec.MethodName,
				ResourceName: auditLogsSourceSpec.ResourceName,
				ServiceName:  auditLogsSourceSpec.ServiceName,
			},
			allowed: false,
		},
		"Secret.Key changed": {
			orig: &auditLogsSourceSpec,
			updated: CloudAuditLogsSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: auditLogsSourceSpec.PubSubSpec.Secret.Name,
						},
						Key: "some-other-key",
					},
					Project: auditLogsSourceSpec.PubSubSpec.Project,
					SourceSpec: duckv1.SourceSpec{
						Sink: auditLogsSourceSpec.PubSubSpec.Sink,
					},
				},
				MethodName:   auditLogsSourceSpec.MethodName,
				ResourceName: auditLogsSourceSpec.ResourceName,
				ServiceName:  auditLogsSourceSpec.ServiceName,
			},
			allowed: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *CloudAuditLogsSource

			if tc.orig != nil {
				if spec, ok := tc.orig.(*CloudAuditLogsSourceSpec); ok {
					orig = &CloudAuditLogsSource{
						Spec: *spec,
					}
				}
			}
			updated := &CloudAuditLogsSource{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
