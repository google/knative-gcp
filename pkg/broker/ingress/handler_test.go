package ingress

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/client/test"
	"github.com/google/go-cmp/cmp"
)

// TestHandlerChannelInbound uses a channel based mock client for both inbound and decouple.
func TestHandlerChannelInbound(t *testing.T) {
	inbound, ic := test.NewMockReceiverClient(t, 1)
	decouple, dc := test.NewMockSenderClient(t, 1)
	_, cleanup := createAndStartIngress(t, inbound, decouple)
	defer cleanup()

	input := createTestEvent()
	// Send an event to the inbound receiver client.
	ic <- input
	// Retrieve the event from the decouple sink.
	output := <-dc

	if dif := cmp.Diff(input, output); dif != "" {
		t.Errorf("Output event doesn't match input, dif: %v", dif)
	}
}

// createAndStartIngress creates an ingress and calls its Start() method in a goroutine.
func createAndStartIngress(t *testing.T, inbound cloudevents.Client, decouple DecoupleSink) (h *handler, cleanup func()) {
	ctx := context.Background()
	h, err := NewHandler(ctx,
		WithInboundClient(inbound),
		WithDecoupleSink(decouple))
	if err != nil {
		t.Fatalf("Failed to create ingress handler: %+v", err)
	}
	go h.Start(ctx)
	cleanup = func() {
		// Any cleanup steps should go here. For now none.
	}
	return h, cleanup
}

func createTestEvent() cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID("test-id")
	event.SetSource("test-source")
	event.SetType("test-type")
	return event
}
