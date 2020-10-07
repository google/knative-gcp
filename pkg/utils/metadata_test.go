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

package utils

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

func TestProjectIDOrDefault(t *testing.T) {
	testCases := map[string]struct {
		want  string
		data  testingMetadataClient.TestClientData
		input string
		env   string
		error bool
	}{
		"project id exists": {
			want:  "testing-project",
			data:  testingMetadataClient.TestClientData{},
			input: "testing-project",
			error: false,
		},
		"Successfully get project id from env": {
			want:  "testing-project",
			data:  testingMetadataClient.TestClientData{},
			env:   "testing-project",
			input: "",
			error: false,
		},
		"Successfully get project id from client": {
			want:  testingMetadataClient.FakeProjectID,
			data:  testingMetadataClient.TestClientData{},
			input: "",
			error: false,
		},
		"project id doesn't exist, get project id failed": {
			want: "",
			data: testingMetadataClient.TestClientData{
				ProjectIDErr: fmt.Errorf("error getting project id"),
			},
			input: "",
			error: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			// Save and restore changed variables
			origEnv := projectIDFromEnv
			defer func() { projectIDFromEnv = origEnv }()
			orig := defaultMetadataClientCreator
			defer func() { defaultMetadataClientCreator = orig }()

			projectIDFromEnv = tc.env
			defaultMetadataClientCreator = func() metadataClient.Client {
				return testingMetadataClient.NewTestClient(tc.data)
			}
			got, err := ProjectIDOrDefault(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
			if tc.error != (err != nil) {
				t.Fatalf("Unexpected validation failure. Got %v", err)
			}
		})
	}
}

func TestClusterName(t *testing.T) {
	testCases := map[string]struct {
		want  string
		data  testingMetadataClient.TestClientData
		input string
		error bool
	}{
		"cluster name exists": {
			want:  "testing-cluster-name",
			data:  testingMetadataClient.TestClientData{},
			input: "testing-cluster-name",
			error: false,
		},
		"cluster name doesn't exist, successfully get cluster name": {
			want:  testingMetadataClient.FakeClusterName,
			data:  testingMetadataClient.TestClientData{},
			input: "",
			error: false,
		},
		"cluster name doesn't exist, get cluster name failed": {
			want: "",
			data: testingMetadataClient.TestClientData{
				ClusterNameErr: fmt.Errorf("get project id failed"),
			},
			input: "",
			error: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client := testingMetadataClient.NewTestClient(tc.data)
			got, err := ClusterName(tc.input, client)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
			if tc.error != (err != nil) {
				t.Fatalf("Unexpected validation failure. Got %v", err)
			}
		})
	}
}
