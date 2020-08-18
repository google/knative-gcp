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

package testing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/extensions"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/broker/eventutil"
	"github.com/google/uuid"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

type serverCfg struct {
	server *httptest.Server
	client *cehttp.Protocol
}

// Helper provides helper functions to facilitate handler pool testing.
type Helper struct {
	// The test pubsub server.
	// PubsubServer.Messages can be called to retrieve all published raw
	// pubsub messages.
	PubsubServer *pstest.Server
	// The pubsub client connected to the test pubsub server.
	// Can be used to operate pubsub resources.
	PubsubClient *pubsub.Client
	// The cloudevents pubsub protocol backed by the test pubsub client.
	// Can be used to send/receive events from the test pubsub server.
	CePubsub *cepubsub.Protocol
	// The targets config maintained by this helper.
	Targets config.Targets

	pubsubConn *grpc.ClientConn

	// The internal map that maps each target to its fake consumer server.
	consumers map[string]*serverCfg
	// The internal map that maps each broker to its fake broker ingress server.
	ingresses map[string]*serverCfg
}

// Close cleans up all resources.
func (h *Helper) Close() {
	h.CePubsub.Close(context.TODO())
	h.PubsubClient.Close()
	h.pubsubConn.Close()
	h.PubsubServer.Close()
	for _, fc := range h.consumers {
		fc.server.Close()
	}
	for _, ing := range h.ingresses {
		ing.server.Close()
	}
}

// NewHelper creates a new helper.
func NewHelper(ctx context.Context, projectID string) (*Helper, error) {
	srv := pstest.NewServer()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	c, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}
	ceps, err := cepubsub.New(ctx,
		cepubsub.WithClient(c),
		cepubsub.WithReceiveSettings(&pubsub.DefaultReceiveSettings),
	)
	if err != nil {
		return nil, err
	}
	return &Helper{
		PubsubServer: srv,
		PubsubClient: c,
		CePubsub:     ceps,
		pubsubConn:   conn,
		Targets:      memory.NewEmptyTargets(),
		consumers:    make(map[string]*serverCfg),
		ingresses:    make(map[string]*serverCfg),
	}, nil
}

// GenerateBroker generates a broker in the given namespace with random broker name.
// The following test resources will also be created.
// 1. The broker decouple topic and subscription.
// 2. The broker ingress server.
func (h *Helper) GenerateBroker(ctx context.Context, t *testing.T, namespace string) *config.Broker {
	t.Helper()

	// Create an empty broker config.
	bn := "br-" + uuid.New().String()
	h.Targets.MutateBroker(namespace, bn, func(bm config.BrokerMutation) {})
	return h.RenewBroker(ctx, t, config.BrokerKey(namespace, bn))
}

// RenewBroker generates new test resources for an existing broker.
func (h *Helper) RenewBroker(ctx context.Context, t *testing.T, brokerKey string) *config.Broker {
	t.Helper()

	b, ok := h.Targets.GetBrokerByKey(brokerKey)
	if !ok {
		t.Fatalf("broker with key %q doesn't exist", brokerKey)
	}

	rid := uuid.New().String()
	topic := "decouple-topic-" + rid
	sub := "decouple-sub-" + rid

	// Create decouple topic/subscription.
	tt, err := h.PubsubClient.CreateTopic(ctx, topic)
	if err != nil {
		t.Fatalf("failed to create test broker decouple topic: %v", err)
	}
	if _, err := h.PubsubClient.CreateSubscription(ctx, sub, pubsub.SubscriptionConfig{Topic: tt}); err != nil {
		t.Fatalf("failed to create test broker decouple subscription: %v", err)
	}

	// Create broker ingress server.
	ceClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create test target cloudevents client: %v", err)
	}
	brokerIngSvr := httptest.NewServer(ceClient)

	h.Targets.MutateBroker(b.Namespace, b.Name, func(bm config.BrokerMutation) {
		bm.SetDecoupleQueue(&config.Queue{
			Topic:        topic,
			Subscription: sub,
		})
		bm.SetAddress(brokerIngSvr.URL)
		bm.SetState(config.State_READY)
	})

	b, ok = h.Targets.GetBrokerByKey(brokerKey)
	if !ok {
		t.Fatalf("failed to save test broker: %s", brokerKey)
	}

	// Clean up existing ingress server if any.
	if cfg, ok := h.ingresses[b.Key()]; ok {
		cfg.server.Close()
	}

	h.ingresses[b.Key()] = &serverCfg{
		server: brokerIngSvr,
		client: ceClient,
	}

	return b
}

// DeleteBroker deletes the broker by key. It also cleans up test resources used by
// the broker.
func (h *Helper) DeleteBroker(ctx context.Context, t *testing.T, brokerKey string) {
	t.Helper()

	b, ok := h.Targets.GetBrokerByKey(brokerKey)
	if !ok {
		// The broker is no longer exists.
		return
	}

	if err := h.PubsubClient.Subscription(b.DecoupleQueue.Subscription).Delete(ctx); err != nil {
		t.Fatalf("failed to delete broker decouple subscription: %v", err)
	}
	if err := h.PubsubClient.Topic(b.DecoupleQueue.Topic).Delete(ctx); err != nil {
		t.Fatalf("failed to delete broker decouple topic: %v", err)
	}

	h.Targets.MutateBroker(b.Namespace, b.Name, func(bm config.BrokerMutation) {
		bm.Delete()
	})

	if ing, ok := h.ingresses[brokerKey]; ok {
		ing.server.Close()
		delete(h.consumers, brokerKey)
	}
}

// GenerateTarget generates a target for the broker with a random name.
// The following test resources will also be created:
// 1. The target retry topic/subscription.
// 2. The subscriber server.
func (h *Helper) GenerateTarget(ctx context.Context, t *testing.T, brokerKey string, filters map[string]string) *config.Target {
	t.Helper()
	tn := "tr-" + uuid.New().String()
	b, ok := h.Targets.GetBrokerByKey(brokerKey)
	if !ok {
		t.Fatalf("broker with key %q doesn't exist", brokerKey)
	}

	testTarget := &config.Target{
		Name:             tn,
		Namespace:        b.Namespace,
		Broker:           b.Name,
		FilterAttributes: filters,
		State:            config.State_READY,
	}

	h.Targets.MutateBroker(b.Namespace, b.Name, func(bm config.BrokerMutation) {
		bm.UpsertTargets(testTarget)
	})

	return h.RenewTarget(ctx, t, testTarget.Key())
}

// RenewTarget generates new test resources for a target.
func (h *Helper) RenewTarget(ctx context.Context, t *testing.T, targetKey string) *config.Target {
	target, ok := h.Targets.GetTargetByKey(targetKey)
	if !ok {
		t.Fatalf("target with key %q doesn't exist", targetKey)
	}

	rid := uuid.New().String()
	topic := "retry-topic-" + rid
	sub := "retry-sub-" + rid

	// Create target retry topic/subscription.
	tt, err := h.PubsubClient.CreateTopic(ctx, topic)
	if err != nil {
		t.Fatalf("failed to create test target retry topic: %v", err)
	}
	if _, err := h.PubsubClient.CreateSubscription(ctx, sub, pubsub.SubscriptionConfig{Topic: tt}); err != nil {
		t.Fatalf("failed to create test target retry subscription: %v", err)
	}

	// Create subscriber server.
	ceClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create test target cloudevents client: %v", err)
	}
	targetSvr := httptest.NewServer(ceClient)

	target.RetryQueue = &config.Queue{
		Topic:        topic,
		Subscription: sub,
	}
	target.Address = targetSvr.URL

	h.Targets.MutateBroker(target.Namespace, target.Broker, func(bm config.BrokerMutation) {
		bm.UpsertTargets(target)
	})

	// Clean up existing ingress server if any.
	if cfg, ok := h.consumers[target.Key()]; ok {
		cfg.server.Close()
	}

	h.consumers[target.Key()] = &serverCfg{
		server: targetSvr,
		client: ceClient,
	}

	return target
}

// DeleteTarget deletes a target and test resources used by it.
func (h *Helper) DeleteTarget(ctx context.Context, t *testing.T, targetKey string) {
	t.Helper()

	target, ok := h.Targets.GetTargetByKey(targetKey)
	if !ok {
		// The broker is no longer exists.
		return
	}

	if err := h.PubsubClient.Subscription(target.RetryQueue.Subscription).Delete(ctx); err != nil {
		t.Fatalf("failed to delete target retry subscription: %v", err)
	}
	if err := h.PubsubClient.Topic(target.RetryQueue.Topic).Delete(ctx); err != nil {
		t.Fatalf("failed to delete target retry topic: %v", err)
	}

	if consumer, ok := h.consumers[targetKey]; ok {
		consumer.server.Close()
		delete(h.consumers, targetKey)
	}

	h.Targets.MutateBroker(target.Namespace, target.Broker, func(bm config.BrokerMutation) {
		bm.DeleteTargets(target)
	})
}

// SendEventToDecoupleQueue sends the given event to the decouple queue of the given broker.
func (h *Helper) SendEventToDecoupleQueue(ctx context.Context, t *testing.T, brokerKey string, event *event.Event) {
	t.Helper()
	b, ok := h.Targets.GetBrokerByKey(brokerKey)
	if !ok {
		t.Fatalf("broker with key %q doesn't exist", brokerKey)
	}

	ctx = cecontext.WithTopic(ctx, b.DecoupleQueue.Topic)
	if err := h.CePubsub.Send(ctx, binding.ToMessage(event)); err != nil {
		t.Fatalf("failed to seed event to broker (key=%q) decouple queue: %v", brokerKey, err)
	}
}

// SendEventToRetryQueue sends the given event to the retry queue of the given target.
func (h *Helper) SendEventToRetryQueue(ctx context.Context, t *testing.T, targetKey string, event *event.Event) {
	t.Helper()
	target, ok := h.Targets.GetTargetByKey(targetKey)
	if !ok {
		t.Fatalf("target with key %q doesn't exist", targetKey)
	}

	ctx = cecontext.WithTopic(ctx, target.RetryQueue.Topic)
	if err := h.CePubsub.Send(ctx, binding.ToMessage(event)); err != nil {
		t.Fatalf("failed to seed event to target (key=%q) retry queue: %v", targetKey, err)
	}
}

// VerifyNextBrokerIngressEvent verifies the next event the broker ingress receives.
// If wantEvent is nil, then it means such an event is not expected.
// This function is blocking and should be invoked in a separate goroutine with context timeout.
func (h *Helper) VerifyNextBrokerIngressEvent(ctx context.Context, t *testing.T, brokerKey string, wantEvent *event.Event) {
	t.Helper()

	bIng, ok := h.ingresses[brokerKey]
	if !ok {
		t.Fatalf("broker with key %q doesn't exist", brokerKey)
	}

	// On timeout or receiving an event, the defer function verifies the event in the end.
	var gotEvent *event.Event
	defer func() {
		assertEvent(t, wantEvent, gotEvent, fmt.Sprintf("broker (key=%q)", brokerKey))
	}()

	msg, err := bIng.client.Receive(ctx)
	if err != nil {
		// In case Receive is stopped.
		if wantEvent != nil {
			t.Errorf("Unexpected error receiving event: %v", err)
		}
		return
	}
	defer msg.Finish(nil)
	gotEvent, err = binding.ToEvent(ctx, msg)
	if err != nil {
		t.Errorf("broker (key=%q) received invalid cloudevent: %v", brokerKey, err)
	}
}

// VerifyNextTargetEvent verifies the next event the subscriber receives.
// If wantEvent is nil, then it means such an event is not expected.
// This function is blocking and should be invoked in a separate goroutine with context timeout.
func (h *Helper) VerifyNextTargetEvent(ctx context.Context, t *testing.T, targetKey string, wantEvent *event.Event) {
	t.Helper()
	h.VerifyAndRespondNextTargetEvent(ctx, t, targetKey, wantEvent, nil, http.StatusOK, 0)
}

// VerifyNextTargetEventAndDelayResp verifies the next event the subscriber receives
// but not respond a success infinitely.
func (h *Helper) VerifyNextTargetEventAndDelayResp(ctx context.Context, t *testing.T, targetKey string, wantEvent *event.Event, delay time.Duration) {
	t.Helper()
	h.VerifyAndRespondNextTargetEvent(ctx, t, targetKey, wantEvent, nil, http.StatusOK, delay)
}

// VerifyAndRespondNextTargetEvent verifies the next event the subscriber receives and replies with the given parameters.
// If wantEvent is nil, then it means such an event is not expected.
// This function is blocking and should be invoked in a separate goroutine with context timeout.
func (h *Helper) VerifyAndRespondNextTargetEvent(ctx context.Context, t *testing.T, targetKey string, wantEvent, replyEvent *event.Event, statusCode int, delay time.Duration) {
	t.Helper()

	// Subscribers should not receive any event with hops.
	var wantEventCopy *event.Event
	if wantEvent != nil {
		copy := wantEvent.Clone()
		wantEventCopy = &copy
		eventutil.DeleteRemainingHops(ctx, wantEventCopy)
	}

	consumer, ok := h.consumers[targetKey]
	if !ok {
		t.Errorf("target with key %q doesn't exist", targetKey)
	}

	// On timeout or receiving an event, the defer function verifies the event in the end.
	var gotEvent *event.Event
	defer func() {
		assertEvent(t, wantEventCopy, gotEvent, fmt.Sprintf("target (key=%q)", targetKey))
	}()

	msg, respFn, err := consumer.client.Respond(ctx)
	if err != nil {
		// In case Receive is stopped.
		if wantEvent != nil {
			t.Errorf("Unexpected error receiving event: %v", err)
		}
		return
	}
	gotEvent, err = binding.ToEvent(ctx, msg)
	if err != nil {
		t.Errorf("target (key=%q) received invalid cloudevent: %v", targetKey, err)
	}

	time.Sleep(delay)

	var replyMsg binding.Message
	if replyEvent != nil {
		replyMsg = binding.ToMessage(replyEvent)
	}
	if err := respFn(ctx, replyMsg, &cehttp.Result{StatusCode: statusCode}); err != nil {
		t.Errorf("unexpected error from responding target (key=%q) event: %v", targetKey, err)
	}
	msg.Finish(nil)
}

// VerifyNextTargetRetryEvent verifies the next event the target retry queue receives.
// Calling this function will also ack the next event.
// If wantEvent is nil, then it means such an event is not expected.
// This function is blocking and should be invoked in a separate goroutine with context timeout.
func (h *Helper) VerifyNextTargetRetryEvent(ctx context.Context, t *testing.T, targetKey string, wantEvent *event.Event) {
	t.Helper()
	target, ok := h.Targets.GetTargetByKey(targetKey)
	if !ok {
		t.Fatalf("target with key %q doesn't exist", targetKey)
	}

	var gotEvent *event.Event
	defer func() {
		t.Helper()
		assertEvent(t, wantEvent, gotEvent, fmt.Sprintf("target (key=%q)", targetKey))
	}()

	// Creates a temp pubsub client to pull the retry subscription.
	psTmp, err := cepubsub.New(ctx,
		cepubsub.WithClient(h.PubsubClient),
		cepubsub.WithTopicID(target.RetryQueue.Topic),
		cepubsub.WithSubscriptionID(target.RetryQueue.Subscription),
		cepubsub.WithReceiveSettings(&pubsub.DefaultReceiveSettings),
	)
	if err != nil {
		t.Fatalf("failed to create temporary pubsub protocol to receive retry events: %v", err)
	}
	defer psTmp.Close(ctx)
	go psTmp.OpenInbound(ctx)

	msg, err := psTmp.Receive(ctx)
	if err != nil {
		// In case Receive is stopped.
		if wantEvent != nil {
			t.Errorf("Unexpected error receiving event: %v", err)
		}
		return
	}
	msg.Finish(nil)
	gotEvent, err = binding.ToEvent(ctx, msg)
	if err != nil {
		t.Errorf("target (key=%q) received invalid cloudevent: %v", targetKey, err)
	}
}

func assertEvent(t *testing.T, want, got *event.Event, msg string) {
	t.Helper()
	if got != nil && want != nil {
		// Clone events so that we don't modify the original copy.
		gotCopy, wantCopy := got.Clone(), want.Clone()
		got, want = &gotCopy, &wantCopy

		// Ignore time.
		got.SetTime(want.Time())
		// Ignore traceparent.
		got.SetExtension(extensions.TraceParentExtension, nil)

		// Compare hops explicitly because
		// cloudevents client sometimes treat hops value as string internally.
		gotHops, _ := eventutil.GetRemainingHops(context.Background(), got)
		wantHops, _ := eventutil.GetRemainingHops(context.Background(), want)
		if gotHops != wantHops {
			t.Errorf("%s event hops got=%d, want=%d", msg, gotHops, wantHops)
		}
		eventutil.DeleteRemainingHops(context.Background(), want)
		eventutil.DeleteRemainingHops(context.Background(), got)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("%s received event (-want,+got): %v", msg, diff)
	}
}
