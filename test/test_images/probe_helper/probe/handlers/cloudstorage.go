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

package handlers

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
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

	// bucketExtension is the CloudEvent extension in which want the probe to
	// manipulate Cloud Storage objects.
	bucketExtension = "bucket"
)

func NewCloudStorageSourceProbe(storageClient *storage.Client) *CloudStorageSourceProbe {
	return &CloudStorageSourceProbe{
		storageClient:  storageClient,
		receivedEvents: utils.NewSyncReceivedEvents(),
	}
}

// CloudStorageSourceProbe is the base probe type for probe requests in the
// CloudStorageSource probes. Since all of the CloudStorageSource probes share
// the same Receive logic, they all inherit it from this object.
type CloudStorageSourceProbe struct {
	// The storage client used in the CloudStorageSource
	storageClient *storage.Client

	// The map of received events to be tracked by the forwarder and receiver
	receivedEvents *utils.SyncReceivedEvents
}

// CloudStorageSourceCreateProbe is the probe handler for probe requests
// in the CloudStorageSource create probe.
type CloudStorageSourceCreateProbe struct {
	*CloudStorageSourceProbe
}

// CloudStorageSourceUpdateMetadataProbe is the probe handler for probe requests
// in the CloudStorageSource update-metadata probe.
type CloudStorageSourceUpdateMetadataProbe struct {
	*CloudStorageSourceProbe
}

// CloudStorageSourceArchiveProbe is the probe handler for probe requests in the
// CloudStorageSource archive probe.
type CloudStorageSourceArchiveProbe struct {
	*CloudStorageSourceProbe
}

// CloudStorageSourceDeleteProbe is the probe handler for probe requests in the
// CloudStorageSource delete probe.
type CloudStorageSourceDeleteProbe struct {
	*CloudStorageSourceProbe
}

// Forward writes an object to Cloud Storage in order to generate a notification
// event.
func (p *CloudStorageSourceCreateProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe writes the event as an object to a given Cloud Storage bucket.
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := p.storageClient.Bucket(fmt.Sprint(bucket))
	objectID := event.ID()[len(event.Type())+1:]
	object := bucketHandle.Object(objectID)
	logging.FromContext(ctx).Infow("Writing object to cloud storage bucket", zap.String("object", objectID), zap.String("bucket", fmt.Sprint(bucket)))
	if err := object.NewWriter(ctx).Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Forward modifies a Cloud Storage object's metadata in order to generate a
// notification event.
func (p *CloudStorageSourceUpdateMetadataProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe modifies an object's metadata.
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := p.storageClient.Bucket(fmt.Sprint(bucket))
	objectID := event.ID()[len(event.Type())+1:]
	object := bucketHandle.Object(objectID)
	objectAttrs := storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			"some-key": "Metadata updated!",
		},
	}
	logging.FromContext(ctx).Infow("Updating object metadata in cloud storage bucket", zap.String("object", objectID), zap.String("bucket", fmt.Sprint(bucket)))
	if _, err := object.Update(ctx, objectAttrs); err != nil {
		return fmt.Errorf("Failed to update object metadata: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Forward archives a Cloud Storage object in order to generate a notification event.
func (p *CloudStorageSourceArchiveProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The storage client updates the object's storage class to ARCHIVE.
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := p.storageClient.Bucket(fmt.Sprint(bucket))
	objectID := event.ID()[len(event.Type())+1:]
	object := bucketHandle.Object(objectID)
	w := object.NewWriter(ctx)
	w.ObjectAttrs.StorageClass = "ARCHIVE"
	logging.FromContext(ctx).Infow("Archiving object in cloud storage bucket", zap.String("object", objectID), zap.String("bucket", fmt.Sprint(bucket)))
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Forward deletes a Cloud Storage object in order to generate a notification event.
func (p *CloudStorageSourceDeleteProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// Deleting a specific version of an object deletes it forever.
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	bucketHandle := p.storageClient.Bucket(fmt.Sprint(bucket))
	objectID := event.ID()[len(event.Type())+1:]
	object := bucketHandle.Object(objectID)
	objectAttrs, err := object.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get object attributes: %v", err)
	}
	logging.FromContext(ctx).Infow("Deleting object in cloud storage bucket", zap.String("object", objectID), zap.String("bucket", fmt.Sprint(bucket)))
	if err := object.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
		return fmt.Errorf("Failed to delete object: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Receive closes the receiver channel associated with the Cloud Storage notification event.
func (p *CloudStorageSourceProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// The original event is written as an identifiable object to a bucket.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: google.cloud.storage.object.v1.finalized
	//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
	//     subject: objects/cloudstoragesource-probe-create-fc2638d1-fcae-4889-9fa1-14a08cb05fc4
	//     id: 1529343217463053
	//     time: 2020-09-14T17:18:40.984Z
	//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
	//     datacontenttype: application/json
	//   Data,
	//     { ... }
	var eventID string
	if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &eventID); err != nil {
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
	eventID = fmt.Sprintf("%s-%s", forwardType, eventID)
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension]), eventID)
	if err := p.receivedEvents.SignalReceiverChannel(channelID); err != nil {
		return err
	}
	logging.FromContext(ctx).Info("Successfully received CloudStorageSource probe event")
	return nil
}
