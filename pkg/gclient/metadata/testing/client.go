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

package testing

import (
	"github.com/google/knative-gcp/pkg/gclient/metadata"
)

var (
	clusterNameAttr = "cluster-name"
	FakeClusterName = "fake-cluster-name"
	FakeProjectID   = "fake-project-id"
)

type testMetadataClient struct {
}

func NewTestClient() metadata.Client {
	return &testMetadataClient{}
}

func (c *testMetadataClient) InstanceAttributeValue(attr string) (string, error) {
	if attr == clusterNameAttr {
		return FakeClusterName, nil
	} else {
		return "", nil
	}
}

func (m *testMetadataClient) ProjectID() (string, error) {
	return FakeProjectID, nil
}

// Assume this process is always running on Google Compute Engine
func (m *testMetadataClient) OnGCE() bool {
	return true
}
