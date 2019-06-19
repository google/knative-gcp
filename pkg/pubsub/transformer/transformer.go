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

package transformer

import (
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"github.com/google/uuid"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/kncloudevents"
)

// PushMessage represents the format Pub/Sub uses to push events.
type PushMessage struct {
	// ID identifies this message.
	// This ID is assigned by the server and is populated for Messages obtained from a subscription.
	// This field is read-only.
	ID string `json:"id,omitempty"`

	// Data is the actual data in the message.
	Data []byte `json:"data,omitempty"`

	// Attributes represents the key-value pairs the current message
	// is labelled with.
	Attributes map[string]string `json:"attributes,omitempty"`

	// The time at which the message was published.
	// This is populated by the server for Messages obtained from a subscription.
	// This field is read-only.
	PublishTime time.Time `json:"publish_time,omitempty"`
}

// Transformer implements the Pub/Sub transformer to convert generic Cloud
// Pub/Sub events into compatible formats.
type Transformer struct {
	// Mode is the mode to transform the event.
	Mode string

	// client is the cloudevents client to use to receive events.
	client cloudevents.Client
}

func (a *Transformer) Start(ctx context.Context) error {
	var err error

	// Receive Events on Pub/Sub.
	if a.client == nil {
		if a.client, err = kncloudevents.NewDefaultClient(); err != nil {
			return fmt.Errorf("failed to create cloudevent client: %s", err.Error())
		}
	}

	return a.client.StartReceiver(ctx, a.receive)
}

func (a *Transformer) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()))

	resp.RespondWith(200, r)

	return nil
}
