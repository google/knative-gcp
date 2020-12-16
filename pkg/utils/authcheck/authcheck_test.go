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
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
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
			name:      "authentication check failed, using empty authType",
			authType:  "empty",
			wantError: true,
			setEnv:    false,
		},
		{
			name:      "authentication check failed, using secret authType",
			authType:  Secret,
			wantError: true,
			setEnv:    true,
		},
		{
			name:      "authentication check failed, using workload-identity-gsa authType",
			authType:  WorkloadIdentityGSA,
			wantError: true,
			setEnv:    true,
		},
		{
			name:      "authentication check succeeded, using workload-identity-gsa authType",
			authType:  WorkloadIdentityGSA,
			wantError: false,
			setEnv:    true,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.setEnv {
				os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "empty")
				defer os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
			}

			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if tc.wantError {
					rw.WriteHeader(http.StatusServiceUnavailable)
				} else {
					rw.WriteHeader(http.StatusOK)
				}
			}))

			defer server.Close()

			authCheck := &defaultAuthenticationCheck{
				authType: tc.authType,
				client:   server.Client(),
				url:      server.URL,
			}

			err := authCheck.Check(ctx)

			if diff := cmp.Diff(tc.wantError, err != nil); diff != "" {
				t.Error("unexpected error (-want, +got) = ", diff)
			}
		})
	}
}
