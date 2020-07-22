/*
Copyright 2019 Google LLC

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

package resources

import (
	"github.com/google/knative-gcp/pkg/utils/naming"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
)

// GenerateTopicName generates a topic name for the storage. This refers to the underlying Pub/Sub topic, and not our
// Topic resource.
func GenerateTopicName(storage *v1.CloudStorageSource) string {
	return naming.TruncatedPubsubResourceName("cre-src", storage.Namespace, storage.Name, storage.UID)
}
