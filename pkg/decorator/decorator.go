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

package decorator

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go"

	"github.com/google/knative-gcp/pkg/kncloudevents"
	"github.com/google/knative-gcp/pkg/reconciler/decorator/resources"
)

// Decorator implements a Decorator to add attributes to a cloudevent and forward to a sink.
type Decorator struct {
	ExtensionsBased64 string `envconfig:"K_CE_EXTENSIONS" required:"true"`
	Sink              string `envconfig:"K_SINK" required:"true"`

	// extensions is the converted ExtensionsBased64 value.
	extensions map[string]string
	// inbound is the cloudevents client to use to receive events.
	inbound cloudevents.Client
	// outbound is the cloudevents client to use to send events.
	outbound cloudevents.Client
}

func (a *Decorator) Start(ctx context.Context) error {
	var err error

	// Receive events on HTTP.
	if a.inbound == nil {
		if a.inbound, err = kncloudevents.NewDefaultClient(); err != nil {
			return fmt.Errorf("failed to create inbound cloudevent client: %s", err.Error())
		}
	}

	// Send Events on HTTP to sink.
	if a.outbound == nil {
		if a.outbound, err = kncloudevents.NewDefaultClient(a.Sink); err != nil {
			return fmt.Errorf("failed to create outbound cloudevent client: %s", err.Error())
		}
	}

	a.extensions = resources.MakeDecoratorExtensionsMap(a.ExtensionsBased64)

	return a.inbound.StartReceiver(ctx, a.receive)
}

func (a *Decorator) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {

	for k, v := range a.extensions {
		event.SetExtension(k, v)
	}

	if r, err := a.outbound.Send(ctx, event); err != nil {
		return err
	} else if r != nil {
		resp.RespondWith(200, r)
	}

	return nil
}
