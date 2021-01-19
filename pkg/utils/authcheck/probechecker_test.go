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
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/knative-gcp/pkg/logging"
	pkgtesting "github.com/google/knative-gcp/pkg/testing"
)

func TestProbeCheckResult(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		wantStatusCode int
	}{
		{
			name:           "probe check got a failure result",
			err:            errors.New("induced error"),
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "probe check got a success result",
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Get a free port.
			port := pkgtesting.GetFreePort(t)

			logger := logging.FromContext(ctx)
			probeChecker := ProbeChecker{
				logger:    logger,
				port:      port,
				authCheck: &FakeAuthenticationCheck{Err: tc.err},
			}
			go probeChecker.Start(ctx)

			time.Sleep(1 * time.Second)

			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/healthz", port), nil)
			if err != nil {
				t.Fatal("Failed to create probe check request:", err)
			}

			client := http.DefaultClient
			resp, err := client.Do(req)

			if err != nil {
				t.Fatal("Failed to execute probe check:", err)
				return
			}
			if tc.wantStatusCode != resp.StatusCode {
				t.Errorf("unexpected probe check status code want %d, got %d)", tc.wantStatusCode, resp.StatusCode)
			}
		})
	}
}
