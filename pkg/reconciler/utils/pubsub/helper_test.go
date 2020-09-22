package pubsub

import (
	"testing"

	. "knative.dev/pkg/reconciler/testing"
)

func TestNewDefaultPubsubClient(t *testing.T) {
	ctx, _ := SetupFakeContext(t)
	_, err := NewDefaultPubsubClient(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create default pubsub client %v", err)
	}
}
