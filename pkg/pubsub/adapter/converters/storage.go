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
)

var (
	// Mapping of GCS eventTypes to CloudEvent types.
	storageEventTypes = map[string]string{
		"OBJECT_FINALIZE":        "google.storage.object.finalize",
		"OBJECT_ARCHIVE":         "google.storage.object.archive",
		"OBJECT_DELETE":          "google.storage.object.delete",
		"OBJECT_METADATA_UPDATE": "google.storage.object.metadataUpdate",
	}
)

const (
	defaultEventType = "google.storage"
	sourcePrefix     = "//storage.googleapis.com/buckets/"
)

func storageSource(bucket string) string {
	return fmt.Sprintf("%s/%s", sourcePrefix, bucket)
}

func convertStorage(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil pubsub message")
	}

	tx := cepubsub.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent()
	event.SetID(tx.ID)
	event.SetTime(tx.PublishTime)
	if msg.Attributes != nil {
		if val, ok := msg.Attributes["bucketId"]; ok {
			delete(msg.Attributes, "bucketId")
			event.SetSource(storageSource(val))
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
				logging.FromContext(ctx).Desugar().Debug("Unknown eventType, using default", zap.String("eventType", eventType), zap.String("default", defaultEventType))
				event.SetType(defaultEventType)
			}
		} else {
			return nil, fmt.Errorf("received event did not have eventType")
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
