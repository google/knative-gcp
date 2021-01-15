/*
Copyright 2021 Google LLC

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

package clients

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"knative.dev/pkg/network"

	pkgtesting "github.com/google/knative-gcp/pkg/testing"
)

func TestNewHTTPMessageReceiverWithChecker(t *testing.T) {
	testCases := []struct {
		name           string
		path           string
		header         http.Header
		wantStatusCode int
	}{
		{
			name: "receiver receives a request from kubelet probe with /healthz path",
			path: "/healthz",
			header: http.Header{
				"User-Agent": []string{network.KubeProbeUAPrefix},
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "receiver receives a request from kubelet probe with /non path",
			path: "/non",
			header: http.Header{
				"User-Agent": []string{network.KubeProbeUAPrefix},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "receiver receives a normal request",
			wantStatusCode: http.StatusAccepted,
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
			receiver := NewHTTPMessageReceiverWithChecker(Port(port), "")
			go receiver.StartListen(ctx, &testHandler{})

			time.Sleep(1 * time.Second)

			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, tc.path), nil)
			req.Header = tc.header
			if err != nil {
				t.Fatal("Failed to create a http request:", err)
			}

			client := http.DefaultClient
			resp, err := client.Do(req)

			if err != nil {
				t.Fatal("Failed to execute HTTPMessageReceiverWithChecker:", err)
				return
			}
			if tc.wantStatusCode != resp.StatusCode {
				t.Errorf("unexpected response status code want %d, got %d)", tc.wantStatusCode, resp.StatusCode)
			}
		})
	}
}

func TestNewHTTPMessageReceiver(t *testing.T) {
	testCases := []struct {
		name           string
		path           string
		header         http.Header
		wantStatusCode int
	}{
		{
			name: "receiver receives a request from kubelet probe with /healthz path",
			path: "/healthz",
			header: http.Header{
				"User-Agent": []string{network.KubeProbeUAPrefix},
			},
			// Without checker, NewHTTPMessageReceiver does not differentiate kubelet probe request from different path.
			// It will write StatusOK regardless of path.
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "receiver receives a normal request",
			wantStatusCode: http.StatusAccepted,
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
			receiver := NewHTTPMessageReceiver(Port(port))
			go receiver.StartListen(ctx, &testHandler{})

			time.Sleep(1 * time.Second)

			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, tc.path), nil)
			req.Header = tc.header
			if err != nil {
				t.Fatal("Failed to create a http request:", err)
			}

			client := http.DefaultClient
			resp, err := client.Do(req)

			if err != nil {
				t.Fatal("Failed to execute HTTPMessageReceiver:", err)
				return
			}
			if tc.wantStatusCode != resp.StatusCode {
				t.Errorf("unexpected response status code want %d, got %d)", tc.wantStatusCode, resp.StatusCode)
			}
		})
	}
}

type testHandler struct{}

func (th *testHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusAccepted)
}
