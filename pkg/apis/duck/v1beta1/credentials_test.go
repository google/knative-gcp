/*
Copyright 2020 Google LLC.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	logtesting "knative.dev/pkg/logging/testing"
)

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
		secret:         DefaultGoogleCloudSecretSelector(),
		serviceAccount: "",
		wantErr:        false,
	}, {
		name: "invalid secret, and nil service account",
		secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: DefaultSecretName,
			},
		},
		serviceAccount: "",
		wantErr:        true,
	}, {
		name: "invalid secret, and nil service account",
		secret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{},
			Key:                  defaultSecretKey,
		},
		serviceAccount: "",
		wantErr:        true,
	}, {
		name:           "nil secret, and valid service account",
		secret:         nil,
		serviceAccount: "test123@test123.iam.gserviceaccount.com",
		wantErr:        false,
	}, {
		name:           "nil secret, and invalid service account",
		secret:         nil,
		serviceAccount: "test@test",
		wantErr:        true,
	}, {
		name:           "secret and service account exist at the same time",
		secret:         DefaultGoogleCloudSecretSelector(),
		serviceAccount: "test@test.iam.gserviceaccount.com",
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
