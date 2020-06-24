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
	"cloud.google.com/go/pubsub"
	"context"
	"errors"
	"fmt"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
)

var (
	// Mapping of GCS eventTypes to CloudEvent types.
	storageEventTypes = map[string]string{
		"OBJECT_FINALIZE":        v1beta1.CloudStorageSourceFinalize,
		"OBJECT_ARCHIVE":         v1beta1.CloudStorageSourceArchive,
		"OBJECT_DELETE":          v1beta1.CloudStorageSourceDelete,
		"OBJECT_METADATA_UPDATE": v1beta1.CloudStorageSourceMetadataUpdate,
	}
)

const (
	// Schema extracted from https://raw.githubusercontent.com/googleapis/google-api-go-client/master/storage/v1/storage-api.json.
	// TODO find the public google endpoint we should use to point to the schema and avoid hosting it ourselves.
	//  The link above is tied to the go-client, and it seems not to be a valid json schema.
	storageSchemaUrl      = "https://raw.githubusercontent.com/google/knative-gcp/master/schemas/storage/schema.json"
)

func convertCloudStorage(ctx context.Context, msg *pubsub.Message) (*cev2.Event, error) {
	event := cev2.NewEvent(cev2.VersionV1)
	event.SetID(msg.ID)
	event.SetTime(msg.PublishTime)
	event.SetDataSchema(storageSchemaUrl)

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
