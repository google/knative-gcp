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

package probe

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

const (
	// CloudStorageSourceCreateProbeEventType is the CloudEvent type of forward
	// CloudStorageSource create probes.
	CloudStorageSourceCreateProbeEventType = "cloudstoragesource-probe-create"

	// CloudStorageSourceUpdateMetadataProbeEventType is the CloudEvent type of forward
	// CloudStorageSource update-metadata probes.
	CloudStorageSourceUpdateMetadataProbeEventType = "cloudstoragesource-probe-update-metadata"

	// CloudStorageSourceArchiveProbeEventType is the CloudEvent type of forward
	// CloudStorageSource archive probes.
	CloudStorageSourceArchiveProbeEventType = "cloudstoragesource-probe-archive"

	// CloudStorageSourceDeleteProbeEventType is the CloudEvent type of forward
	// CloudStorageSource delete probes.
	CloudStorageSourceDeleteProbeEventType = "cloudstoragesource-probe-delete"

	bucketExtension = "bucket"
)

type CloudStorageSourceProbe struct{}

// CloudStorageSourceCreateProbe is the probe handler for probe requests
// in the CloudStorageSource create probe.
type CloudStorageSourceCreateProbe struct {
	*CloudStorageSourceProbe
}

var cloudStorageSourceCreateProbe Handler = &CloudStorageSourceCreateProbe{}

type CloudStorageSourceUpdateMetadataProbe struct {
	*CloudStorageSourceProbe
}

var cloudStorageSourceUpdateMetadataProbe Handler = &CloudStorageSourceUpdateMetadataProbe{}

type CloudStorageSourceArchiveProbe struct {
	*CloudStorageSourceProbe
}

var cloudStorageSourceArchiveProbe Handler = &CloudStorageSourceArchiveProbe{}

type CloudStorageSourceDeleteProbe struct {
	*CloudStorageSourceProbe
}

var cloudStorageSourceDeleteProbe Handler = &CloudStorageSourceDeleteProbe{}

func (p *CloudStorageSourceCreateProbe) Forward(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Get the receiver channel
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], event.ID())
	receiverChannel, cleanupFunc, err := CreateReceiverChannel(ctx, ph, channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe writes the event as an object to a given Cloud Storage bucket.
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := ph.StorageClient.Bucket(fmt.Sprint(bucket))
	obj := bucketHandle.Object(event.ID()[len(event.Type())+1:])
	if err := obj.NewWriter(ctx).Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}

	return WaitOnReceiverChannel(ctx, receiverChannel)
}

func (p *CloudStorageSourceUpdateMetadataProbe) Forward(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Get the receiver channel
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], event.ID())
	receiverChannel, cleanupFunc, err := CreateReceiverChannel(ctx, ph, channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := ph.StorageClient.Bucket(fmt.Sprint(bucket))
	obj := bucketHandle.Object(event.ID()[len(event.Type())+1:])
	objectAttrs := storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			"some-key": "Metadata updated!",
		},
	}
	if _, err := obj.Update(ctx, objectAttrs); err != nil {
		return fmt.Errorf("Failed to update object metadata: %v", err)
	}

	return WaitOnReceiverChannel(ctx, receiverChannel)
}

func (p *CloudStorageSourceArchiveProbe) Forward(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Get the receiver channel
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], event.ID())
	receiverChannel, cleanupFunc, err := CreateReceiverChannel(ctx, ph, channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := ph.StorageClient.Bucket(fmt.Sprint(bucket))
	obj := bucketHandle.Object(event.ID()[len(event.Type())+1:])
	// The storage client updates the object's storage class to ARCHIVE.
	w := obj.NewWriter(ctx)
	w.ObjectAttrs.StorageClass = "ARCHIVE"
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}

	return WaitOnReceiverChannel(ctx, receiverChannel)
}

func (p *CloudStorageSourceDeleteProbe) Forward(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Get the receiver channel
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], event.ID())
	receiverChannel, cleanupFunc, err := CreateReceiverChannel(ctx, ph, channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := ph.StorageClient.Bucket(fmt.Sprint(bucket))
	obj := bucketHandle.Object(event.ID()[len(event.Type())+1:])
	// Deleting a specific version of an object deletes it forever.
	objectAttrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get object attributes: %v", err)
	}
	if err := obj.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
		return fmt.Errorf("Failed to delete object: %v", err)
	}

	return WaitOnReceiverChannel(ctx, receiverChannel)
}

func (p *CloudStorageSourceProbe) Receive(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// The original event is written as an identifiable object to a bucket.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: google.cloud.storage.object.v1.finalized
	//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
	//     subject: objects/cloudstoragesource-probe-fc2638d1-fcae-4889-9fa1-14a08cb05fc4
	//     id: 1529343217463053
	//     time: 2020-09-14T17:18:40.984Z
	//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
	//     datacontenttype: application/json
	//   Data,
	//     { ... }
	var channelID string
	if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
		return fmt.Errorf("Failed to extract probe event ID from Cloud Storage event subject: %v", err)
	}
	var forwardType string
	switch event.Type() {
	case schemasv1.CloudStorageObjectFinalizedEventType:
		forwardType = CloudStorageSourceCreateProbeEventType
	case schemasv1.CloudStorageObjectMetadataUpdatedEventType:
		forwardType = CloudStorageSourceUpdateMetadataProbeEventType
	case schemasv1.CloudStorageObjectArchivedEventType:
		forwardType = CloudStorageSourceArchiveProbeEventType
	case schemasv1.CloudStorageObjectDeletedEventType:
		forwardType = CloudStorageSourceDeleteProbeEventType
	default:
		return fmt.Errorf("Unrecognized Cloud Storage event type: %s", event.Type())
	}
	channelID = fmt.Sprintf("%s/%s-%s", event.Extensions()[probeEventTargetServiceExtension], forwardType, channelID)
	return CloseReceiverChannel(ctx, ph, channelID)
}
