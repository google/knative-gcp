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
	"testing"

	"github.com/google/go-cmp/cmp"

	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

func TestProjectID(t *testing.T) {
	testCases := map[string]struct {
		want  string
		input string
	}{
		"cluster name exists": {
			want:  "testing-project",
			input: "testing-project",
		},
		"cluster name doesn't exist": {
			want:  testingMetadataClient.FakeProjectID,
			input: "",
		},
	}
	client := testingMetadataClient.NewTestClient()
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got, _ := ProjectID(tc.input, client)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}

func TestClusterName(t *testing.T) {
	testCases := map[string]struct {
		want  string
		input string
	}{
		"cluster name exists": {
			want:  "testing-cluster-name",
			input: "testing-cluster-name",
		},
		"cluster name doesn't exist": {
			want:  testingMetadataClient.FakeClusterName,
			input: "",
		},
	}
	client := testingMetadataClient.NewTestClient()
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got, _ := ClusterName(tc.input, client)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}
