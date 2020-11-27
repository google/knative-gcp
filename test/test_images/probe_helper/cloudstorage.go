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

package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

// Probe event goes here
const (
	bucketExtension = "bucket"
)

type CloudStorageSourceForwardProbe struct {
	ProbeInterface
	event     cloudevents.Event
	channelID string
	bucket    *storage.BucketHandle
}

type CloudStorageSourceCreateForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

type CloudStorageSourceUpdateMetadataForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

type CloudStorageSourceArchiveForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

type CloudStorageSourceDeleteForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

func CloudStorageSourceForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (*CloudStorageSourceForwardProbe, error) {
	//requestHost, ok := event.Extensions()[ProbeEventRequestHostExtension]
	//if !ok {
	//	return nil, fmt.Errorf("Failed to read '%s' extension", ProbeEventRequestHostExtension)
	//}
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return nil, fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	probe := &CloudStorageSourceForwardProbe{
		event:     event,
		channelID: event.Type() + "-" + event.ID(),
		bucket:    ph.storageClient.Bucket(fmt.Sprint(bucket)),
	}
	return probe, nil
}

func CloudStorageSourceCreateForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	probe, err := CloudStorageSourceForwardProbeConstructor(ph, event)
	if err != nil {
		return nil, err
	}
	return CloudStorageSourceCreateForwardProbe{
		CloudStorageSourceForwardProbe: probe,
	}, nil
}

func CloudStorageSourceUpdateMetadataForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	probe, err := CloudStorageSourceForwardProbeConstructor(ph, event)
	if err != nil {
		return nil, err
	}
	return CloudStorageSourceUpdateMetadataForwardProbe{
		CloudStorageSourceForwardProbe: probe,
	}, nil
}

func CloudStorageSourceArchiveForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	probe, err := CloudStorageSourceForwardProbeConstructor(ph, event)
	if err != nil {
		return nil, err
	}
	return CloudStorageSourceArchiveForwardProbe{
		CloudStorageSourceForwardProbe: probe,
	}, nil
}

func CloudStorageSourceDeleteForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	probe, err := CloudStorageSourceForwardProbeConstructor(ph, event)
	if err != nil {
		return nil, err
	}
	return CloudStorageSourceDeleteForwardProbe{
		CloudStorageSourceForwardProbe: probe,
	}, nil
}

func (p CloudStorageSourceForwardProbe) ChannelID() string {
	return p.channelID
}

func (p CloudStorageSourceCreateForwardProbe) Handle(ctx context.Context) error {
	// The storage client writes an object named as the event ID.
	obj := p.bucket.Object(p.event.ID())
	if err := obj.NewWriter(ctx).Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}
	return nil
}

func (p CloudStorageSourceUpdateMetadataForwardProbe) Handle(ctx context.Context) error {
	// The storage client updates the object metadata.
	objectAttrs := storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			"some-key": "Metadata updated!",
		},
	}
	obj := p.bucket.Object(p.event.ID())
	if _, err := obj.Update(ctx, objectAttrs); err != nil {
		return fmt.Errorf("Failed to update object metadata: %v", err)
	}
	return nil
}

func (p CloudStorageSourceArchiveForwardProbe) Handle(ctx context.Context) error {
	// The storage client updates the object's storage class to ARCHIVE.
	obj := p.bucket.Object(p.event.ID())
	w := obj.NewWriter(ctx)
	w.ObjectAttrs.StorageClass = "ARCHIVE"
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}
	return nil
}

func (p CloudStorageSourceDeleteForwardProbe) Handle(ctx context.Context) error {
	// Deleting a specific version of an object deletes it forever.
	obj := p.bucket.Object(p.event.ID())
	objectAttrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get object attributes: %v", err)
	}
	if err := obj.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
		return fmt.Errorf("Failed to delete object: %v", err)
	}
	return nil
}

// Receiver event goes here
type CloudStorageSourceReceiveProbe struct {
	ProbeInterface
	channelID string
}

func CloudStorageSourceReceiveProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	// The original event is written as an identifiable object to a bucket.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: google.cloud.storage.object.v1.finalized
	//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
	//     subject: objects/fc2638d1-fcae-4889-9fa1-14a08cb05fc4
	//     id: 1529343217463053
	//     time: 2020-09-14T17:18:40.984Z
	//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
	//     datacontenttype: application/json
	//   Data,
	//     { ... }
	var channelID string
	if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
		return nil, fmt.Errorf("Failed to extract probe event ID from Cloud Storage event subject: %v", err)
	}
	switch event.Type() {
	case schemasv1.CloudStorageObjectFinalizedEventType:
		channelID = "cloudstoragesource-probe-create-" + channelID
	case schemasv1.CloudStorageObjectMetadataUpdatedEventType:
		channelID = "cloudstoragesource-probe-update-metadata-" + channelID
	case schemasv1.CloudStorageObjectArchivedEventType:
		channelID = "cloudstoragesource-probe-archive-" + channelID
	case schemasv1.CloudStorageObjectDeletedEventType:
		channelID = "cloudstoragesource-probe-delete-" + channelID
	default:
		return nil, fmt.Errorf("Unrecognized Cloud Storage event type: %v", event.Type())
	}
	return CloudStorageSourceReceiveProbe{
		channelID: channelID,
	}, nil
}

func (p CloudStorageSourceReceiveProbe) ChannelID() string {
	return p.channelID
}
