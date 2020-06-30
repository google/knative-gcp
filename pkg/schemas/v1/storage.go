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

package v1

import "fmt"

const (
	CloudStorageObjectFinalizedEventType       = "google.cloud.storage.object.v1.finalized"
	CloudStorageObjectArchivedEventType        = "google.cloud.storage.object.v1.archived"
	CloudStorageObjectDeletedEventType         = "google.cloud.storage.object.v1.deleted"
	CloudStorageObjectMetadataUpdatedEventType = "google.cloud.storage.object.v1.metadataUpdated"
	CloudStorageEventDataSchema                = "https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto"
)

func CloudStorageEventSource(bucket string) string {
	return fmt.Sprintf("//storage.googleapis.com/projects/_/buckets/%s", bucket)
}

func CloudStorageEventSubject(object string) string {
	return fmt.Sprintf("objects/%s", object)
}
