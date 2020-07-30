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

package eventutil

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"
)

const (
	// Intentionally make it short because the additional attribute
	// increases Pubsub message size and could incur additional cost.
	HopsAttribute = "kgcphops"
)

// UpdateRemainingHops update an event with proper remaining hops.
// 1. If the event doesn't have an existing hops value or have an invalid hops value,
// it puts the preemptive hops in the event.
// 2. If the event has an existing valid hops value,
// it decrements it by 1.
func UpdateRemainingHops(ctx context.Context, event *event.Event, preemptiveHops int32) {
	hops, ok := GetRemainingHops(ctx, event)
	if ok {
		// Decrement hops.
		hops = hops - 1
		if hops < 0 {
			hops = 0
		}
	} else {
		logging.FromContext(ctx).Debug("Remaining hops not found in event, defaulting to the preemptive value.",
			zap.String("event.id", event.ID()),
			zap.Int32(HopsAttribute, preemptiveHops),
		)
		hops = preemptiveHops
	}

	event.SetExtension(HopsAttribute, hops)
}

type SetRemainingHopsTransformer int32

func (h SetRemainingHopsTransformer) Transform(_ binding.MessageMetadataReader, out binding.MessageMetadataWriter) error {
	out.SetExtension(HopsAttribute, int32(h))
	return nil
}

// GetRemainingHops returns the remaining hops of the event if it presents.
// If there is no existing hops value or an invalid one, (0, false) will be returned.
func GetRemainingHops(ctx context.Context, event *event.Event) (int32, bool) {
	hopsRaw, ok := event.Extensions()[HopsAttribute]
	if !ok {
		return 0, false
	}
	hops, err := cetypes.ToInteger(hopsRaw)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to convert existing hops value into integer, regarding it as there is no hops value.",
			zap.String("event.id", event.ID()),
			zap.Any(HopsAttribute, hopsRaw),
			zap.Error(err),
		)
		return 0, false
	}
	return hops, true
}

// DeleteRemainingHops deletes hops from the event extensions.
func DeleteRemainingHops(_ context.Context, event *event.Event) {
	event.SetExtension(HopsAttribute, nil)
}
