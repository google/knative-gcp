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

// Package iam provides interfaces and wrappers around the google iam client.
package metadata

// Client matches the interface exposed by metadata.Client
type Client interface {
	// InstanceAttributeValue returns the value of the provided VM instance attribute.
	// See https://godoc.org/cloud.google.com/compute/metadata#Client.ProjectID
	InstanceAttributeValue(attr string) (string, error)

	// ProjectID returns the current instance's project ID string.
	// See https://godoc.org/cloud.google.com/compute/metadata#Client.InstanceAttributeValue
	ProjectID() (string, error)

	// OnGCE reports whether this process is running on Google Compute Engine.
	// See https://godoc.org/cloud.google.com/compute/metadata#OnGCE
	OnGCE() bool
}
