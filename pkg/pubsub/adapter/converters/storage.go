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

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"knative.dev/pkg/logging"
)

func ConvertStorage(ctx context.Context, m transport.Message, err error) (*cloudevents.Event, error) {
	logger := logging.FromContext(ctx)
	logger.Debug("Converting event from transport.")
	if msg, ok := m.(*cepubsub.Message); ok {
		tx := cepubsub.TransportContextFrom(ctx)
		// Make a new event and convert the message payload.
		event := cloudevents.NewEvent()
		event.SetID(tx.ID)
		event.SetTime(tx.PublishTime)
		if msg.Attributes != nil {
			if val, ok := msg.Attributes["notificationConfig"]; ok {
				event.SetSource(val)
			} else {
				return nil, fmt.Errorf("received event did not have notificationConfig")
			}
		}
		event.SetType("google.storage")
		event.SetDataContentType(*cloudevents.StringOfApplicationJSON())
		event.Data = msg.Data
		event.DataEncoded = true
		// Attributes are extensions.
		if msg.Attributes != nil && len(msg.Attributes) > 0 {
			for k, v := range msg.Attributes {
				event.SetExtension(k, v)
			}
		}

		logger.Infof("Context: %q", event.Context.String())
		return &event, nil
	}
	return nil, fmt.Errorf("not a pubsub message, bailing")
}
