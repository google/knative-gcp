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

package metadata

import (
	"net"
	"net/http"
	"time"

	"cloud.google.com/go/compute/metadata"
)

var (
	// Default HTTP Client from "cloud.google.com/go/compute/metadata"
	defaultClient = &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			ResponseHeaderTimeout: 2 * time.Second,
		},
	}
)

// metadataClient wraps metadata.Client.
// It is the client that will be used everywhere except unit tests.
type metadataClient struct {
	metadata *metadata.Client
}

func (m *metadataClient) InstanceAttributeValue(attr string) (string, error) {
	return m.metadata.InstanceAttributeValue(attr)
}

func (m *metadataClient) ProjectID() (string, error) {
	return m.metadata.ProjectID()
}

func (m *metadataClient) OnGCE() bool {
	return metadata.OnGCE()
}

func NewMetadataClient(httpClient *http.Client) Client {
	return &metadataClient{
		metadata: metadata.NewClient(httpClient),
	}
}

func NewDefaultMetadataClient() Client {
	return NewMetadataClient(defaultClient)
}
