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

package authtype

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

func TestGetAuthTypeForWorkloadIdentity(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		objects      []runtime.Object
		args         AuthTypeArgs
		wantAuthType string
		wantError    error
	}{
		{
			name: "successfully get authType for workload identity",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS,
					pkgtesting.WithServiceAccountAnnotation("name"),
				),
			},
			args:         serviceAccountArgs,
			wantAuthType: "workload-identity-gsa",
			wantError:    nil,
		},
		{
			name:         "error get authType, service account doesn't exist",
			objects:      []runtime.Object{},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration, " +
				"can't find Kubernetes Service Account " + serviceAccountName),
		},
		{
			name: "error get authType, service account doesn't have required annotation",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(serviceAccountName, testNS),
			},
			args:         serviceAccountArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("using Workload Identity for authentication configuration, " +
				"Kubernetes Service Account " + serviceAccountName + " doesn't have the required annotation"),
		},
		{
			name: "successfully get authType for secret (not in broker namespace)",
			objects: []runtime.Object{
				pkgtesting.NewSecret(secretName, testNS),
			},
			args:         secretArgs,
			wantAuthType: "secret",
			wantError:    nil,
		},
		{
			name: "error get authType, secret doesn't exist in broker namespace",
			objects: []runtime.Object{
				pkgtesting.NewServiceAccount(brokerServiceAccountName, testBrokerNS),
			},
			args:         brokerArgs,
			wantAuthType: "",
			wantError: fmt.Errorf("authentication is not configured, " +
				"Secret doesn't present, ServiceAccountName doesn't have required annotation"),
		},
		{
			name: "error get authType, secret doesn't have required key in broker namespace",
			objects: []runtime.Object{
				pkgtesting.NewSecret(brokerSecretName, testBrokerNS),
			},
			args:         brokerArgs,
			wantAuthType: "secret",
			wantError: fmt.Errorf("using Secret for authentication configuration, " +
				"Kubernests Secret " + brokerSecretName + " doesn't have required key key.json"),
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
			wantAuthType: "secret",
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

			gotAuthType, gotError := GetAuthType(ctx, lister.GetServiceAccountLister(), lister.GetSecretLister(), tc.args)

			if diff := cmp.Diff(tc.wantAuthType, gotAuthType); diff != "" {
				t.Errorf("unexpected authType (-want, +got) = %v", diff)
			}
			if tc.wantError != nil {
				if gotError == nil {
					t.Errorf("unexpected authType error (-want %v, +got %v)", tc.wantError.Error(), "")
				} else if diff := cmp.Diff(tc.wantError.Error(), gotError.Error()); diff != "" {
					t.Errorf("unexpected authType (-want, +got) = %v", diff)
				}
			}
		})
	}
}
