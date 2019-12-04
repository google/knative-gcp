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
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

func convertPubsub(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	tx := pubsubcontext.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(tx.ID)
	event.SetTime(tx.PublishTime)
	event.SetSource(v1alpha1.PubSubEventSource(tx.Project, tx.Topic))
	// We do not know the content type and we do not want to inspect the payload,
	// thus we set this generic one.
	event.SetDataContentType("application/octet-stream")
	event.SetType(v1alpha1.PubSubPublish)
	// Set the schema if it comes as an attribute.
	if val, ok := msg.Attributes["schema"]; ok {
		delete(msg.Attributes, "schema")
		event.SetDataSchema(val)
	}
	// Set the mode to be an extension attribute.
	event.SetExtension("knativecemode", string(sendMode))
	event.Data = msg.Data
	event.DataEncoded = true
	// Attributes are extensions.
	if msg.Attributes != nil && len(msg.Attributes) > 0 {
		for k, v := range msg.Attributes {
			// V1 attributes MUST consist of lower-case letters ('a' to 'z') or digits ('0' to '9').
			event.SetExtension(strings.ToLower(k), v)
		}
	}
	return &event, nil
}
