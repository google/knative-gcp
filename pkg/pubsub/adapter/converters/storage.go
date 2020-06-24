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

package converters

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
)

var (
	// Mapping of GCS eventTypes to CloudEvent types.
	storageEventTypes = map[string]string{
		"OBJECT_FINALIZE":        v1beta1.CloudStorageSourceObjectFinalizedEventType,
		"OBJECT_ARCHIVE":         v1beta1.CloudStorageSourceObjectArchivedEventType,
		"OBJECT_DELETE":          v1beta1.CloudStorageSourceObjectDeletedEventType,
		"OBJECT_METADATA_UPDATE": v1beta1.CloudStorageSourceObjectMetadataUpdatedEventType,
	}
)

func convertCloudStorage(ctx context.Context, msg *pubsub.Message) (*cev2.Event, error) {
	event := cev2.NewEvent(cev2.VersionV1)
	event.SetID(msg.ID)
	event.SetTime(msg.PublishTime)
	event.SetDataSchema(v1beta1.CloudStorageSourceEventDataSchema)

	// TODO: figure out if we want to continue to add these as extensions.
	if val, ok := msg.Attributes["bucketId"]; ok {
		event.SetSource(v1beta1.CloudStorageSourceEventSource(val))
	} else {
		return nil, errors.New("received event did not have bucketId")
	}
	if val, ok := msg.Attributes["objectId"]; ok {
		event.SetSubject(val)
	} else {
		return nil, errors.New("received event did not have objectId")
	}

	if val, ok := msg.Attributes["eventType"]; ok {
		if eventType, ok := storageEventTypes[val]; ok {
			event.SetType(eventType)
		} else {
			return nil, fmt.Errorf("unknown event type %s", val)
		}
	} else {
		return nil, errors.New("received event did not have eventType")
	}

	if err := event.SetData(cev2.ApplicationJSON, msg.Data); err != nil {
		return nil, err
	}
	return &event, nil
}
