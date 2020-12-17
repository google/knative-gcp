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

package authcheck

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	pkgtesting "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS                   = "test"
	testBrokerNS             = "cloud-run-events"
	serviceAccountName       = "test-ksa"
	secretName               = "test-secret"
	brokerServiceAccountName = "broker"
	brokerSecretName         = "google-broker-key"
)

var (
	serviceAccountArgs = AuthTypeArgs{
		Namespace:          testNS,
		ServiceAccountName: serviceAccountName,
	}

	secretArgs = AuthTypeArgs{
		Namespace: testNS,
		Secret: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: secretName,
			},
			Key: "key.json",
		},
	}

	brokerArgs = AuthTypeArgs{
		Namespace:          testBrokerNS,
		ServiceAccountName: brokerServiceAccountName,
		Secret: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: brokerSecretName,
			},
			Key: "key.json",
		},
	}
)

func TestGetAuthTypeForSources(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		objects      []runtime.Object
		args         AuthTypeArgs
		wantAuthType AuthType
		wantError    error
	}{
		{
			name: "successfully get authType for workload identity",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS,
					pkgtesting.WithServiceAccountAnnotation("service-account-name@project-id.iam.gserviceaccount.com"),
				),
			},
			args:         serviceAccountArgs,
			wantAuthType: WorkloadIdentityGSA,
			wantError:    nil,
		},
		{
			name: "successfully get authType for workload identity, with old style project id",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS,
					pkgtesting.WithServiceAccountAnnotation("longname@project-example.example.com.iam.gserviceaccount.com"),
				),
			},
			args:         serviceAccountArgs,
			wantAuthType: WorkloadIdentityGSA,
			wantError:    nil,
		},
		{
			name: "error get authType, invalid service account without a email-like format",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS,
					pkgtesting.WithServiceAccountAnnotation("name"),
				),
			},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration: name is not a valid Google Service Account " +
				"as the value of Kubernetes Service Account test-ksa for annotation iam.gke.io/gcp-service-account"),
		},
		{
			name: "error get authType, invalid service account with a email-like format but without project id",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS,
					pkgtesting.WithServiceAccountAnnotation("name@.iam.gserviceaccount.com"),
				),
			},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration: name@.iam.gserviceaccount.com is not a valid Google Service Account " +
				"as the value of Kubernetes Service Account test-ksa for annotation iam.gke.io/gcp-service-account"),
		},
		{
			name: "error get authType, invalid service account with a email-like format but name is too short",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS,
					pkgtesting.WithServiceAccountAnnotation("name@project-id.iam.gserviceaccount.com"),
				),
			},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration: name@project-id.iam.gserviceaccount.com is not a valid Google Service Account " +
				"as the value of Kubernetes Service Account test-ksa for annotation iam.gke.io/gcp-service-account"),
		},
		{
			name:         "error get authType, service account doesn't exist",
			objects:      []runtime.Object{},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration: " +
				"can't find Kubernetes Service Account " + serviceAccountName),
		},
		{
			name: "error get authType, service account does not have required annotation",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS),
			},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration: " +
				"the Kubernetes Service Account " + serviceAccountName + " does not have the required annotation"),
		},
		{
			name: "successfully get authType for secret (not in broker namespace)",
			objects: []runtime.Object{
				pkgtesting.NewSecret(secretName, testNS),
			},
			args:         secretArgs,
			wantAuthType: Secret,
			wantError:    nil,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			lister := pkgtesting.NewListers(tc.objects)

			gotAuthType, gotError := GetAuthTypeForSources(ctx, lister.GetServiceAccountLister(), tc.args)

			if diff := cmp.Diff(tc.wantAuthType, gotAuthType); diff != "" {
				t.Error("unexpected authType (-want, +got) = ", diff)
			}
			if tc.wantError != nil {
				if gotError == nil {
					t.Errorf("unexpected authType error (-want %v, +got %v)", tc.wantError.Error(), "")
				} else if diff := cmp.Diff(tc.wantError.Error(), gotError.Error()); diff != "" {
					t.Error("unexpected authType (-want, +got) = ", diff)
				}
			}
		})
	}
}

func TestGetAuthTypeForBrokerCell(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		objects      []runtime.Object
		args         AuthTypeArgs
		wantAuthType AuthType
		wantError    error
	}{
		{
			name: "error get authType, secret doesn't exist in broker namespace",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(brokerServiceAccountName, testBrokerNS),
			},
			args:         brokerArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("authentication is not configured, " +
				"when checking Kubernetes Service Account broker, got error: the Kubernetes Service Account broker does not have the required annotation, " +
				"when checking Kubernetes Secret google-broker-key, got error: can't find Kubernetes Secret google-broker-key"),
		},
		{
			name: "error get authType, secret does not have required key in broker namespace",
			objects: []runtime.Object{
				pkgtesting.NewSecret(brokerSecretName, testBrokerNS),
			},
			args:         brokerArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("authentication is not configured, " +
				"when checking Kubernetes Service Account broker, got error: can't find Kubernetes Service Account broker, " +
				"when checking Kubernetes Secret google-broker-key, got error: the Kubernetes Secret google-broker-key does not have required key key.json"),
		},
		{
			name: "successfully get authType in broker namespace",
			objects: []runtime.Object{
				pkgtesting.NewSecret(brokerSecretName, testBrokerNS,
					pkgtesting.WithData(map[string][]byte{
						"key.json": make([]byte, 5, 5),
					})),
			},
			args:         brokerArgs,
			wantAuthType: Secret,
			wantError:    nil,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			lister := pkgtesting.NewListers(tc.objects)

			gotAuthType, gotError := GetAuthTypeForBrokerCell(ctx, lister.GetServiceAccountLister(), lister.GetSecretLister(), tc.args)

			if diff := cmp.Diff(tc.wantAuthType, gotAuthType); diff != "" {
				t.Error("unexpected authType (-want, +got) = ", diff)
			}
			if tc.wantError != nil {
				if gotError == nil {
					t.Errorf("unexpected authType error (-want %v, +got %v)", tc.wantError.Error(), "")
				} else if diff := cmp.Diff(tc.wantError.Error(), gotError.Error()); diff != "" {
					t.Error("unexpected authType (-want, +got) = ", diff)
				}
			}
		})
	}
}
