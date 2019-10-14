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
	"fmt"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

var (
	// Mapping of GCS eventTypes to CloudEvent types.
	storageEventTypes = map[string]string{
		"OBJECT_FINALIZE":        v1alpha1.StorageFinalize,
		"OBJECT_ARCHIVE":         v1alpha1.StorageArchive,
		"OBJECT_DELETE":          v1alpha1.StorageDelete,
		"OBJECT_METADATA_UPDATE": v1alpha1.StorageMetadataUpdate,
	}
)

const (
	storageDefaultEventType = "google.storage"
	// Schema extracted from https://raw.githubusercontent.com/googleapis/google-api-go-client/master/storage/v1/storage-api.json.
	// TODO find the public google endpoint we should use to point to the schema and avoid hosting it ourselves.
	//  The link above is tied to the go-client, and it seems not to be a valid json schema.
	storageSchemaUrl = "https://raw.githubusercontent.com/google/knative-gcp/master/schemas/storage/schema.json"
)

func convertStorage(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil pubsub message")
	}

	tx := pubsubcontext.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetID(tx.ID)
	event.SetTime(tx.PublishTime)
	event.SetDataSchema(storageSchemaUrl)
	if msg.Attributes != nil {
		if val, ok := msg.Attributes["bucketId"]; ok {
			delete(msg.Attributes, "bucketId")
			event.SetSource(v1alpha1.StorageEventSource(val))
		} else {
			return nil, fmt.Errorf("received event did not have bucketId")
		}
		if val, ok := msg.Attributes["objectId"]; ok {
			delete(msg.Attributes, "objectId")
			event.SetSubject(val)
		} else {
			// Not setting subject, as it's optional
			logging.FromContext(ctx).Desugar().Debug("received event did not have objectId")
		}
		if val, ok := msg.Attributes["eventType"]; ok {
			delete(msg.Attributes, "eventType")
			if eventType, ok := storageEventTypes[val]; ok {
				event.SetType(eventType)
			} else {
				logging.FromContext(ctx).Desugar().Debug("Unknown eventType, using default", zap.String("eventType", eventType), zap.String("default", storageDefaultEventType))
				event.SetType(storageDefaultEventType)
			}
		} else {
			return nil, fmt.Errorf("received event did not have eventType")
		}
		if _, ok := msg.Attributes["eventTime"]; ok {
			delete(msg.Attributes, "eventTime")
		}
	}
	event.SetDataContentType(*cloudevents.StringOfApplicationJSON())
	event.SetData(msg.Data)
	// Attributes are extensions.
	if msg.Attributes != nil && len(msg.Attributes) > 0 {
		for k, v := range msg.Attributes {
			event.SetExtension(k, v)
		}
	}
	return &event, nil
}
