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
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/pubsub/pstest"
	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"cloud.google.com/go/pubsub"

	"github.com/google/knative-gcp/pkg/broker/config"
)

func CreateTestPubsubClient(ctx context.Context, t *testing.T, projectID string) (*pubsub.Client, func()) {
	t.Helper()
	srv := pstest.NewServer()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial test pubsub connection: %v", err)
	}
	close := func() {
		srv.Close()
		conn.Close()
	}
	c, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("failed to create test pubsub client: %v", err)
	}
	return c, close
}

func GenTestBroker(ctx context.Context, t *testing.T, ps *pubsub.Client) *config.Broker {
	t.Helper()
	tn := "topic-" + uuid.New().String()
	sn := "sub-" + uuid.New().String()

	tt, err := ps.CreateTopic(ctx, tn)
	if err != nil {
		t.Fatalf("failed to create test broker pubsub topic: %v", err)
	}
	if _, err := ps.CreateSubscription(ctx, sn, pubsub.SubscriptionConfig{Topic: tt}); err != nil {
		t.Fatalf("failed to create test broker subscription: %v", err)
	}

	return &config.Broker{
		Name:      "broker-" + uuid.New().String(),
		Namespace: "ns",
		DecoupleQueue: &config.Queue{
			Topic:        tn,
			Subscription: sn,
		},
		State: config.State_READY,
	}
}

func GenTestTarget(ctx context.Context, t *testing.T, ps *pubsub.Client, filters map[string]string) *config.Target {
	t.Helper()
	tn := "topic-" + uuid.New().String()
	sn := "sub-" + uuid.New().String()

	tt, err := ps.CreateTopic(ctx, tn)
	if err != nil {
		t.Fatalf("failed to create test target pubsub topic: %v", err)
	}
	if _, err := ps.CreateSubscription(ctx, sn, pubsub.SubscriptionConfig{Topic: tt}); err != nil {
		t.Fatalf("failed to create test target subscription: %v", err)
	}

	return &config.Target{
		Name:             "target-" + uuid.New().String(),
		Namespace:        "ns",
		FilterAttributes: filters,
		RetryQueue: &config.Queue{
			Topic:        tn,
			Subscription: sn,
		},
		State: config.State_READY,
	}
}

func AddTestTargetToBroker(t *testing.T, targets config.Targets, target *config.Target, broker string) (string, *cehttp.Protocol, func()) {
	t.Helper()

	targetClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create target cloudevents client: %v", err)
	}
	targetSvr := httptest.NewServer(targetClient)
	// Add subscriber URI to target.
	target.Address = targetSvr.URL
	targets.MutateBroker("ns", broker, func(bm config.BrokerMutation) {
		bm.UpsertTargets(target)
	})

	return broker + "/" + target.Name, targetClient, targetSvr.Close
}

func SentEventToTopic(ctx context.Context, t *testing.T, ceps *cepubsub.Protocol, topic string, e *event.Event) {
	ctx = cecontext.WithTopic(ctx, topic)
	if err := ceps.Send(ctx, binding.ToMessage(e)); err != nil {
		t.Errorf("failed to seed event to pubsub: %v", err)
	}
}

func VerifyNextReceivedEvent(ctx context.Context, t *testing.T, receiver string, client *cehttp.Protocol, wantEvent *event.Event, wantCnt int) {
	t.Helper()

	gotCnt := 0
	defer func() {
		if gotCnt != wantCnt {
			t.Errorf("[%s] event received got=%d, want=%d", receiver, gotCnt, wantCnt)
		}
	}()

	msg, err := client.Receive(ctx)
	if err != nil {
		// In case Receive is stopped.
		return
	}
	msg.Finish(nil)
	gotEvent, err := binding.ToEvent(ctx, msg)
	if err != nil {
		t.Errorf("[%s] ingress received message cannot be converted to an event: %v", receiver, err)
	}
	gotCnt++
	// Force the time to be the same so that we can compare easier.
	gotEvent.SetTime(wantEvent.Time())
	if diff := cmp.Diff(wantEvent, gotEvent); diff != "" {
		t.Errorf("[%s] target received event (-want,+got): %v", receiver, diff)
	}
}
