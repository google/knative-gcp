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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/logging"
)

func TestAuthenticationCheck(t *testing.T) {
	testCases := []struct {
		name           string
		authType       AuthType
		wantStatusCode int
		setEnv         bool
	}{
		{
			name:           "authentication check uses empty authType",
			authType:       "",
			wantStatusCode: http.StatusUnauthorized,
			setEnv:         false,
		},
		{
			name:           "authentication check uses secret authType",
			authType:       Secret,
			wantStatusCode: http.StatusUnauthorized,
			setEnv:         true,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			logger := logging.FromContext(ctx)
			response := httptest.NewRecorder()

			if tc.setEnv {
				os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "empty")
			}

			AuthenticationCheck(ctx, logger, tc.authType, response)

			if diff := cmp.Diff(tc.wantStatusCode, response.Result().StatusCode); diff != "" {
				t.Error("unexpected status (-want, +got) = ", diff)
			}
		})
	}
}

func TestAuthenticationCheckForWorkloadIdentityGSA(t *testing.T) {
	resource := "http:/empty"
	wantErr := errors.New("error getting the http response: Get \"http:///empty\": http: no Host in request URL")
	gotErr := AuthenticationCheckForWorkloadIdentityGSA(resource)
	if diff := cmp.Diff(gotErr.Error(), wantErr.Error()); diff != "" {
		t.Error("unexpected status (-want, +got) = ", diff)
	}
}
