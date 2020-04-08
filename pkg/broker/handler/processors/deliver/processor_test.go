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

package deliver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
)

func TestInvalidContext(t *testing.T) {
	p := &Processor{}
	e := event.New()
	err := p.Process(context.Background(), &e)
	if err != handlerctx.ErrBrokerNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrBrokerNotPresent)
	}

	ctx := handlerctx.WithBroker(context.Background(), &config.Broker{})
	err = p.Process(ctx, &e)
	if err != handlerctx.ErrTargetNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrTargetNotPresent)
	}
}

func TestDeliverSuccess(t *testing.T) {
	targetClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create target cloudevents client: %v", err)
	}
	ingressClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create ingress cloudevents client: %v", err)
	}
	requester, err := cev2.NewDefaultClient()
	if err != nil {
		t.Fatalf("failed to create requester cloudevents client: %v", err)
	}
	targetSvr := httptest.NewServer(targetClient)
	defer targetSvr.Close()
	ingressSvr := httptest.NewServer(ingressClient)
	defer ingressSvr.Close()
	p := &Processor{Requester: requester}

	broker := &config.Broker{Namespace: "ns", Name: "broker", Address: ingressSvr.URL}
	target := &config.Target{Namespace: "ns", Name: "target", Broker: "broker", Address: targetSvr.URL}
	ctx := handlerctx.WithBroker(context.Background(), broker)
	ctx = handlerctx.WithTarget(ctx, target)

	origin := event.New()
	origin.SetID("id")
	origin.SetSource("source")
	origin.SetSubject("subject")
	origin.SetType("type")
	origin.SetTime(time.Now())

	reply := origin.Clone()
	reply.SetID("reply")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, resp, err := targetClient.Respond(ctx)
		if err != nil {
			t.Errorf("unexpected error from target receiving event: %v", err)
		}
		if err := resp(ctx, binding.ToMessage(&reply), protocol.ResultACK); err != nil {
			t.Errorf("unexpected error from target responding event: %v", err)
		}
		defer msg.Finish(nil)
		gotEvent, err := binding.ToEvent(ctx, msg)
		if err != nil {
			t.Errorf("target received message cannot be converted to an event: %v", err)
		}
		// Force the time to be the same so that we can compare easier.
		gotEvent.SetTime(origin.Time())
		if diff := cmp.Diff(&origin, gotEvent); diff != "" {
			t.Errorf("target received event (-want,+got): %v", diff)
		}
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := ingressClient.Receive(ctx)
		if err != nil {
			t.Errorf("unexpected error from ingress receiving event: %v", err)
		}
		defer msg.Finish(nil)
		gotEvent, err := binding.ToEvent(ctx, msg)
		if err != nil {
			t.Errorf("ingress received message cannot be converted to an event: %v", err)
		}
		// Force the time to be the same so that we can compare easier.
		if diff := cmp.Diff(&reply, gotEvent); diff != "" {
			t.Errorf("ingress received event (-want,+got): %v", diff)
		}
	}()

	if err := p.Process(ctx, &origin); err != nil {
		t.Errorf("unexpected error from processing: %v", err)
	}
}

func TestDeliverFailure(t *testing.T) {
	targetClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create target cloudevents client: %v", err)
	}
	requester, err := cev2.NewDefaultClient()
	if err != nil {
		t.Fatalf("failed to create requester cloudevents client: %v", err)
	}
	targetSvr := httptest.NewServer(targetClient)
	defer targetSvr.Close()
	p := &Processor{Requester: requester}

	broker := &config.Broker{Namespace: "ns", Name: "broker"}
	target := &config.Target{Namespace: "ns", Name: "target", Broker: "broker", Address: targetSvr.URL}
	ctx := handlerctx.WithBroker(context.Background(), broker)
	ctx = handlerctx.WithTarget(ctx, target)

	origin := event.New()
	origin.SetID("id")
	origin.SetSource("source")
	origin.SetSubject("subject")
	origin.SetType("type")
	origin.SetTime(time.Now())

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, resp, err := targetClient.Respond(ctx)
		if err != nil {
			t.Errorf("unexpected error from target receiving event: %v", err)
		}
		// Due to https://github.com/cloudevents/sdk-go/issues/433
		// it's not possible to use Receive to easily return error.
		if err := resp(ctx, nil, &cehttp.Result{StatusCode: http.StatusInternalServerError}); err != nil {
			t.Errorf("unexpected error from target responding event: %v", err)
		}
		defer msg.Finish(nil)
		gotEvent, err := binding.ToEvent(ctx, msg)
		if err != nil {
			t.Errorf("target received message cannot be converted to an event: %v", err)
		}
		// Force the time to be the same so that we can compare easier.
		gotEvent.SetTime(origin.Time())
		if diff := cmp.Diff(&origin, gotEvent); diff != "" {
			t.Errorf("target received event (-want,+got): %v", diff)
		}
	}()

	if err := p.Process(ctx, &origin); err == nil {
		t.Error("expected error from processing")
	}
}
