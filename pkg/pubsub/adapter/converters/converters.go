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

// Package converters contains pubsub message to cloudevent converters
// used by pubsub-based source.
package converters

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
)

// ModeType is the type for mode enum.
type ModeType string

const (
	// Binary mode is binary encoding.
	Binary ModeType = "binary"
	// Structured mode is structured encoding.
	Structured ModeType = "structured"
	// Push mode emulates Pub/Sub push encoding.
	Push ModeType = "push"
	// DefaultSendMode is the default choice.
	DefaultSendMode = Binary
)

type converterFn func(context.Context, *cepubsub.Message, ModeType) (*cloudevents.Event, error)

// converters is the map for handling Source specific event
// conversions. For example, a GCS event will need to be
// converted differently from the PubSub. The key into
// this map will be "knative-gcp" CloudEvent attribute.
// If there's no such attribute, we assume it's a native
// PubSub message and a default one will be used.
var converters map[string]converterFn

func init() {
	converters = map[string]converterFn{
		AuditLogAdapterType:        convertAuditLog,
		"com.google.cloud.storage": convertStorage,
	}
}

// Convert converts a message off the pubsub format to a source specific if
// there's a registered handler for the type in the converters map.
// If there's no registered handler, a default Pubsub one will be used.
func Convert(ctx context.Context, msg *cepubsub.Message, sendMode ModeType, adapterType string) (*cloudevents.Event, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil pubsub message")
	}
	if c, ok := converters[adapterType]; ok {
		return c(ctx, msg, sendMode)
	}
	if msg.Attributes != nil {
		if val, ok := msg.Attributes["knative-gcp"]; ok {
			delete(msg.Attributes, "knative-gcp")
			if c, ok := converters[val]; ok {
				return c(ctx, msg, sendMode)
			}
		}
	}

	// pubsub is the default one.
	return convertPubsub(ctx, msg, sendMode)
}
