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
	"net/http"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	authchecktesting "github.com/google/knative-gcp/pkg/gclient/authcheck/testing"
)

func TestAuthenticationCheck(t *testing.T) {
	testCases := []struct {
		name       string
		authType   AuthType
		statusCode int
		wantError  bool
		setEnv     bool
	}{
		{
			name:       "authentication check failed, using empty authType",
			authType:   "empty",
			statusCode: http.StatusUnauthorized,
			wantError:  true,
			setEnv:     false,
		},
		{
			name:       "authentication check failed, using secret authType",
			authType:   Secret,
			statusCode: http.StatusUnauthorized,
			wantError:  true,
			setEnv:     true,
		},
		{
			name:       "authentication check failed, using workload-identity-gsa authType",
			authType:   WorkloadIdentityGSA,
			statusCode: http.StatusUnauthorized,
			wantError:  true,
			setEnv:     true,
		},
		{
			name:       "authentication check succeeded",
			authType:   WorkloadIdentityGSA,
			statusCode: http.StatusOK,
			wantError:  false,
			setEnv:     true,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.setEnv {
				os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "empty")
			}

			err := AuthenticationCheck(ctx, tc.authType, authchecktesting.NewFakeAuthCheckClient(tc.statusCode))

			if diff := cmp.Diff(tc.wantError, err != nil); diff != "" {
				t.Error("unexpected error (-want, +got) = ", err)
			}
		})
	}
}
