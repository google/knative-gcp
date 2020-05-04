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
	cetypes "github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"
)

const (
	ttlAttribute = "kngcpbrokerttl"
)

// TTL manages events TTL.
type TTL struct {
	Logger *zap.Logger
}

// UpdateTTL update an event with proper TTL.
// 1. If the event doesn't have an existing TTL or have an invalid TTL,
// it puts the preemptive TTL in the event.
// 2. If the event has an existing valid TTL,
// it decrements it by 1.
func (t *TTL) UpdateTTL(event *event.Event, preemptiveTTL int32) {
	ttl, ok := t.GetTTL(event)
	if ok {
		// Decrement TTL.
		ttl = ttl - 1
		if ttl < 0 {
			ttl = 0
		}
	} else {
		t.Logger.Debug("TTL not found in event, defaulting to the preemptive value.",
			zap.String("event.id", event.ID()),
			zap.Int32(ttlAttribute, preemptiveTTL),
		)
		ttl = preemptiveTTL
	}

	event.SetExtension(ttlAttribute, ttl)
}

// GetTTL returns the existing TTL in the event if it presents.
// If there is no existing TTL or an invalid TTL, (0, false) will be returned.
func (t *TTL) GetTTL(event *event.Event) (int32, bool) {
	ttlRaw, ok := event.Extensions()[ttlAttribute]
	if !ok {
		return 0, false
	}
	ttl, err := cetypes.ToInteger(ttlRaw)
	if err != nil {
		t.Logger.Warn("Failed to convert existing TTL into integer, regarding it as there is no TTL.",
			zap.String("event.id", event.ID()),
			zap.Any(ttlAttribute, ttlRaw),
			zap.Error(err),
		)
		return 0, false
	}
	return ttl, true
}

// DeleteTTL deletes TTL in the event.
func (t *TTL) DeleteTTL(event *event.Event) {
	event.SetExtension(ttlAttribute, nil)
}
