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

	"cloud.google.com/go/pubsub"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
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
)

type ConverterType string

const (
	// The different type of Converters for the different sources.
	CloudPubSub    ConverterType = "pubsub"
	CloudStorage   ConverterType = "storage"
	CloudAuditLogs ConverterType = "auditlogs"
	CloudScheduler ConverterType = "scheduler"
	CloudBuild     ConverterType = "build"
	PubSubPull     ConverterType = "pubsub_pull"
)

type converterFn func(context.Context, *pubsub.Message) (*cev2.Event, error)

type Converter interface {
	Convert(ctx context.Context, msg *pubsub.Message, converterType ConverterType) (*cev2.Event, error)
}

type PubSubConverter struct {
	// converters is the map for handling Source specific event
	// conversions. For example, a GCS event will need to be
	// converted differently than a PubSub one. The key into
	// this map will be the adapter type. If not present,
	// we assume it's a PubSub message and a default
	// one will be used.
	converters map[ConverterType]converterFn
}

func NewPubSubConverter() Converter {
	return &PubSubConverter{
		converters: map[ConverterType]converterFn{
			CloudPubSub:    convertCloudPubSub,
			CloudAuditLogs: convertCloudAuditLogs,
			CloudStorage:   convertCloudStorage,
			CloudScheduler: convertCloudScheduler,
			CloudBuild:     convertCloudBuild,
			PubSubPull:     convertPubSubPull,
		},
	}
}

// Convert converts a message off the pubsub format to a source specific if
// there's a registered handler for the type in the converters map.
// If there's no registered handler, a default Pubsub one will be used.
func (c *PubSubConverter) Convert(ctx context.Context, msg *pubsub.Message, converterType ConverterType) (*cev2.Event, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil pubsub message")
	}
	// Try the converterType, if specified.
	if converterType != "" {
		if c, ok := c.converters[converterType]; ok {
			return c(ctx, msg)
		}
	}

	// No converter, PubSub is the default one.
	return binding.ToEvent(ctx, cepubsub.NewMessage(msg))

}
