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

	cetypes "github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"
)

const (
	hopsAttribute = "kngcpbrokerhopsremaining"
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
			zap.Int32(hopsAttribute, preemptiveHops),
		)
		hops = preemptiveHops
	}

	event.SetExtension(hopsAttribute, hops)
}

// SetRemainingHops sets the remaining hops in the event.
// It ignores any existing hops value.
func SetRemainingHops(ctx context.Context, event *event.Event, hops int32) {
	event.SetExtension(hopsAttribute, hops)
}

// GetRemainingHops returns the remaining hops of the event if it presents.
// If there is no existing hops value or an invalid one, (0, false) will be returned.
func GetRemainingHops(ctx context.Context, event *event.Event) (int32, bool) {
	hopsRaw, ok := event.Extensions()[hopsAttribute]
	if !ok {
		return 0, false
	}
	hops, err := cetypes.ToInteger(hopsRaw)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to convert existing hops value into integer, regarding it as there is no hops value.",
			zap.String("event.id", event.ID()),
			zap.Any(hopsAttribute, hopsRaw),
			zap.Error(err),
		)
		return 0, false
	}
	return hops, true
}

// DeleteRemainingHops deletes hops from the event extensions.
func DeleteRemainingHops(_ context.Context, event *event.Event) {
	event.SetExtension(hopsAttribute, nil)
}
