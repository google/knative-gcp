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
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	authchecktesting "github.com/google/knative-gcp/pkg/gclient/authcheck/testing"
	"github.com/google/knative-gcp/pkg/logging"
)

func TestProbeCheckResult(t *testing.T) {
	testCases := []struct {
		name           string
		inputStatus    int
		wantStatusCode int
	}{
		{
			name:           "probe check got a failure result",
			inputStatus:    http.StatusBadRequest,
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "probe check got a success result",
			inputStatus:    http.StatusAccepted,
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			// Get a free port.
			addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
			if err != nil {
				t.Fatal("Failed to resolve TCP address:", err)
			}
			l, err := net.ListenTCP("tcp", addr)
			if err != nil {
				t.Fatal("Failed to listen TCP:", err)
			}
			l.Close()
			port := l.Addr().(*net.TCPAddr).Port

			logger := logging.FromContext(ctx)
			probeChecker := ProbeChecker{
				logger:          logger,
				port:            port,
				authType:        WorkloadIdentityGSA,
				authCheckClient: authchecktesting.NewFakeAuthCheckClient(tc.inputStatus),
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
			if diff := cmp.Diff(resp.StatusCode, tc.wantStatusCode); diff != "" {
				t.Error("unexpected probe check result (-want, +got) = ", diff)
			}
			cancel()
		})
	}
}
