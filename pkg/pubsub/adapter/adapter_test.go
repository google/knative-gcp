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

package adapter

import (
	"context"
	"knative.dev/pkg/source"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	cepubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
)

type mockStatsReporter struct{}

// TODO: test this more.
func (r *mockStatsReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	return nil
}

// TODO: test this more.
func TestAdapter(t *testing.T) {
	ctx := context.TODO()
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
	}

	_ = a.Start(ctx)
}

// TODO: test this more.
func TestConvert(t *testing.T) {
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
	}

	ctx := cepubsubcontext.WithTransportContext(context.TODO(), cepubsubcontext.NewTransportContext(
		"proj",
		"top",
		"sub",
		"test",
		&pubsub.Message{
			ID: "abc",
			Attributes: map[string]string{
				"test": "me",
			},
		},
	))

	msg := &cepubsub.Message{}
	var err error
	event, gotErr := a.convert(ctx, msg, err)

	_ = event
	_ = gotErr

}

// TODO: test this more.
func TestHTTPClient(t *testing.T) {
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
		SendMode:     "binary",
	}

	_, _ = a.newHTTPClient(context.TODO(), "http://localhost:8080")
}

// TODO: test this more.
func TestReceive(t *testing.T) {
	c, _ := cloudevents.NewDefaultClient()
	r := &mockStatsReporter{}
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
		SendMode:     "binary",

		outbound: c,
		reporter: r,
	}

	ctx := context.Background()
	event := cloudevents.NewEvent()
	event.SetSource("source")
	event.SetType("unit.testing")
	event.SetID("abc-123")
	event.SetDataContentType("application/json")
	_ = event.SetData("testing")

	_ = a.receive(ctx, event, nil)
}
