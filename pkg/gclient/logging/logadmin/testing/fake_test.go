/*
Copyright 2019 Google LLC.

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

package testing

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/logging/logadmin"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateClientInjectedError(t *testing.T) {
	injectedErr := errors.New("injected error")
	ctx := context.Background()
	_, err := TestClientCreator(TestClientConfiguration{CreateClientErr: injectedErr})(ctx, "test-project")
	if err != injectedErr {
		t.Errorf("expected injected create client error %v, got %v", injectedErr, err)
	}
}

func TestCreateSink(t *testing.T) {
	testCases := []struct {
		name         string
		existing     *logadmin.Sink
		sink         *logadmin.Sink
		errCode      codes.Code
		clientConfig TestClientConfiguration
	}{
		{
			name: "create succeeds",
			sink: &logadmin.Sink{
				ID: "test-sink",
			},
		},
		{
			name: "create succeeds other sink",
			existing: &logadmin.Sink{
				ID: "existing-sink",
			},
			sink: &logadmin.Sink{
				ID: "test-sink",
			},
		},
		{
			name: "create already exists",
			existing: &logadmin.Sink{
				ID: "test-sink",
			},
			sink: &logadmin.Sink{
				ID: "test-sink",
			},
			errCode: codes.AlreadyExists,
		},
		{
			name: "create injected error",
			sink: &logadmin.Sink{
				ID: "test-sink",
			},
			errCode: codes.Internal,
			clientConfig: TestClientConfiguration{
				CreateSinkErr: status.Error(codes.Internal, "injected error"),
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := createClient(t, tt.clientConfig, ctx, "test-project")
			if tt.existing != nil {
				if _, err := client.CreateSink(ctx, tt.existing); err != nil {
					t.Errorf("failed to create sink during setup: %v", err)
				}
			}

			created, err := client.CreateSink(ctx, tt.sink)

			if code := status.Code(err); code != tt.errCode {
				t.Errorf("Unexpected error code, wanted %v, got %v", tt.errCode, code)
			}
			if err == nil && tt.errCode == codes.OK {
				actual, err := client.Sink(ctx, tt.sink.ID)
				if err != nil {
					t.Errorf("unable to get sink after creation: %v", err)
				} else if diff := cmp.Diff(created, actual); diff != "" {
					t.Log("Unexpected diff between returned sink and actual sink:")
					t.Log(diff)
					t.Fail()
				}
			}
		})
	}
}

func TestGetSink(t *testing.T) {
	testCases := []struct {
		name         string
		existing     *logadmin.Sink
		sinkID       string
		errCode      codes.Code
		clientConfig TestClientConfiguration
	}{
		{
			name: "get succeeds",
			existing: &logadmin.Sink{
				ID: "test-sink",
			},
			sinkID: "test-sink",
		},
		{
			name:    "get not found",
			sinkID:  "test-sink",
			errCode: codes.NotFound,
		},
		{
			name: "get not found other sink",
			existing: &logadmin.Sink{
				ID: "existing-sink",
			},
			sinkID:  "test-sink",
			errCode: codes.NotFound,
		},
		{
			name: "get injected error",
			existing: &logadmin.Sink{
				ID: "test-sink",
			},
			errCode: codes.Internal,
			clientConfig: TestClientConfiguration{
				SinkErr: status.Error(codes.Internal, "injected error"),
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := createClient(t, tt.clientConfig, ctx, "test-project")
			if tt.existing != nil {
				if _, err := client.CreateSink(ctx, tt.existing); err != nil {
					t.Errorf("failed to create sink during setup: %v", err)
				}
			}

			got, err := client.Sink(ctx, tt.sinkID)

			if code := status.Code(err); code != tt.errCode {
				t.Errorf("unexpected error code, wanted %v, got %v", tt.errCode, code)
			}
			if err == nil && tt.errCode == codes.OK {
				if diff := cmp.Diff(tt.existing, got, cmpopts.IgnoreFields(*got, "WriterIdentity")); diff != "" {
					t.Errorf("Unexpected diff between created sink and returned sink: %v", diff)
				}
			}
		})
	}
}

func TestDeleteSink(t *testing.T) {
	testCases := []struct {
		name         string
		existing     *logadmin.Sink
		sinkID       string
		errCode      codes.Code
		clientConfig TestClientConfiguration
	}{
		{
			name: "delete succeeds",
			existing: &logadmin.Sink{
				ID: "test-sink",
			},
			sinkID: "test-sink",
		},
		{
			name:    "delete not found",
			sinkID:  "test-sink",
			errCode: codes.NotFound,
		},
		{
			name: "delete not found other sink",
			existing: &logadmin.Sink{
				ID: "existing-sink",
			},
			sinkID:  "test-sink",
			errCode: codes.NotFound,
		},
		{
			name: "delete injected error",
			existing: &logadmin.Sink{
				ID: "test-sink",
			},
			errCode: codes.Internal,
			clientConfig: TestClientConfiguration{
				DeleteSinkErr: status.Error(codes.Internal, "injected error"),
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := createClient(t, tt.clientConfig, ctx, "test-project")
			if tt.existing != nil {
				if _, err := client.CreateSink(ctx, tt.existing); err != nil {
					t.Errorf("failed to create sink during setup: %v", err)
				}
			}

			err := client.DeleteSink(ctx, tt.sinkID)

			if code := status.Code(err); code != tt.errCode {
				t.Errorf("unexpected error code, wanted %v, got %v", tt.errCode, code)
			}
			if err == nil && tt.errCode == codes.OK {
				_, err = client.Sink(ctx, tt.sinkID)
				if code := status.Code(err); code != codes.NotFound {
					t.Errorf("expected code NotFound after delete, got %v", code)
				}
			}
		})
	}
}

func createClient(t *testing.T, config TestClientConfiguration, ctx context.Context, parent string) glogadmin.Client {
	client, err := TestClientCreator(config)(ctx, parent)
	if err != nil {
		t.Errorf("client creation failed: %v", err)
	}
	return client
}
