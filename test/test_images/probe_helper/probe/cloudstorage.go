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

// CloudStorageSourceForwardProbe is the probe handler for forward probe requests
// in the CloudStorageSource probe.
type CloudStorageSourceForwardProbe struct {
	Handler
	channelID string
	object    string
	bucket    *storage.BucketHandle
}

// CloudStorageSourceCreateForwardProbe is the probe handler for forward probe requests
// in the CloudStorageSource create probe.
type CloudStorageSourceCreateForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

// CloudStorageSourceUpdateMetadataForwardProbe is the probe handler for forward probe requests
// in the CloudStorageSource update-metadata probe.
type CloudStorageSourceUpdateMetadataForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

// CloudStorageSourceArchiveForwardProbe is the probe handler for forward probe requests
// in the CloudStorageSource archive probe.
type CloudStorageSourceArchiveForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

// CloudStorageSourceDeleteForwardProbe is the probe handler for forward probe requests
// in the CloudStorageSource delete probe.
type CloudStorageSourceDeleteForwardProbe struct {
	*CloudStorageSourceForwardProbe
}

// CloudStorageSourceForwardProbeConstructor builds a new CloudStorageSource forward
// probe handler from a given CloudEvent.
func CloudStorageSourceForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (*CloudStorageSourceForwardProbe, error) {
	bucket, ok := event.Extensions()[bucketExtension]
	if !ok {
		return nil, fmt.Errorf("CloudStorageSource probe event has no '%s' extension", bucketExtension)
	}
	probe := &CloudStorageSourceForwardProbe{
		channelID: fmt.Sprintf("%s/%s", requestHost, event.ID()),
		object:    event.ID()[len(event.Type())+1:],
		bucket:    ph.StorageClient.Bucket(fmt.Sprint(bucket)),
	}
	return probe, nil
}

// CloudStorageSourceCreateForwardProbeConstructor builds a new CloudStorageSource create forward
// probe handler from a given CloudEvent.
func CloudStorageSourceCreateForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	handler, err := CloudStorageSourceForwardProbeConstructor(ph, event, requestHost)
	if err != nil {
		return nil, err
	}
	return &CloudStorageSourceCreateForwardProbe{
		CloudStorageSourceForwardProbe: handler,
	}, nil
}

// CloudStorageSourceUpdateMetadataForwardProbeConstructor builds a new CloudStorageSource update-metadata forward
// probe handler from a given CloudEvent.
func CloudStorageSourceUpdateMetadataForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	handler, err := CloudStorageSourceForwardProbeConstructor(ph, event, requestHost)
	if err != nil {
		return nil, err
	}
	return &CloudStorageSourceUpdateMetadataForwardProbe{
		CloudStorageSourceForwardProbe: handler,
	}, nil
}

// CloudStorageSourceArchiveForwardProbeConstructor builds a new CloudStorageSource archive forward
// probe handler from a given CloudEvent.
func CloudStorageSourceArchiveForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	handler, err := CloudStorageSourceForwardProbeConstructor(ph, event, requestHost)
	if err != nil {
		return nil, err
	}
	return &CloudStorageSourceArchiveForwardProbe{
		CloudStorageSourceForwardProbe: handler,
	}, nil
}

// CloudStorageSourceDeleteForwardProbeConstructor builds a new CloudStorageSource delete forward
// probe handler from a given CloudEvent.
func CloudStorageSourceDeleteForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	handler, err := CloudStorageSourceForwardProbeConstructor(ph, event, requestHost)
	if err != nil {
		return nil, err
	}
	return &CloudStorageSourceDeleteForwardProbe{
		CloudStorageSourceForwardProbe: handler,
	}, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p CloudStorageSourceForwardProbe) ChannelID() string {
	return p.channelID
}

// Handle will create an object in a given Cloud Storage bucket.
func (p CloudStorageSourceCreateForwardProbe) Handle(ctx context.Context) error {
	obj := p.bucket.Object(p.object)
	// The storage client writes an object named as the event ID.
	if err := obj.NewWriter(ctx).Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}
	return nil
}

// Handle will update the metadata of an object in a given Cloud Storage bucket.
func (p CloudStorageSourceUpdateMetadataForwardProbe) Handle(ctx context.Context) error {
	obj := p.bucket.Object(p.object)
	objectAttrs := storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			"some-key": "Metadata updated!",
		},
	}
	if _, err := obj.Update(ctx, objectAttrs); err != nil {
		return fmt.Errorf("Failed to update object metadata: %v", err)
	}
	return nil
}

// Handle will archive an object in a given Cloud Storage bucket.
func (p CloudStorageSourceArchiveForwardProbe) Handle(ctx context.Context) error {
	obj := p.bucket.Object(p.object)
	// The storage client updates the object's storage class to ARCHIVE.
	w := obj.NewWriter(ctx)
	w.ObjectAttrs.StorageClass = "ARCHIVE"
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close storage writer for object finalizing: %v", err)
	}
	return nil
}

// Handle will delete an object in a given Cloud Storage bucket.
func (p CloudStorageSourceDeleteForwardProbe) Handle(ctx context.Context) error {
	obj := p.bucket.Object(p.object)
	// Deleting a specific version of an object deletes it forever.
	objectAttrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get object attributes: %v", err)
	}
	if err := obj.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
		return fmt.Errorf("Failed to delete object: %v", err)
	}
	return nil
}

// CloudStorageSourceReceiveProbe is the probe handler for receiver probe requests
// in the CloudStorageSource probe.
type CloudStorageSourceReceiveProbe struct {
	Handler
	channelID string
}

// CloudStorageSourceReceiveProbeConstructor builds a new CloudStorageSource receiver
// probe handler from a given CloudEvent.
func CloudStorageSourceReceiveProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
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
		return nil, fmt.Errorf("Failed to extract probe event ID from Cloud Storage event subject: %v", err)
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
		return nil, fmt.Errorf("Unrecognized Cloud Storage event type: %s", event.Type())
	}
	return CloudStorageSourceReceiveProbe{
		channelID: fmt.Sprintf("%s/%s-%s", requestHost, forwardType, channelID),
	}, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p CloudStorageSourceReceiveProbe) ChannelID() string {
	return p.channelID
}
