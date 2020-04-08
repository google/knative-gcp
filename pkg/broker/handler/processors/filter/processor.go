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

package filter

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"

	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

// Processor is the processor to filter events based on trigger filters.
type Processor struct {
	processors.BaseProcessor
}

var _ processors.Interface = (*Processor)(nil)

// Process passes the event to the next processor if the event passes the filter.
// Otherwise it simply returns.
func (p *Processor) Process(ctx context.Context, event *event.Event) error {
	target, err := handlerctx.GetTarget(ctx)
	if err != nil {
		return err
	}
	if target.FilterAttributes == nil {
		return p.Next().Process(ctx, event)
	}

	if p.passFilter(ctx, target.FilterAttributes, event) {
		return p.Next().Process(ctx, event)
	}
	logging.FromContext(ctx).Debug("event does not pass filter for target", zap.Any("target", target))
	return nil
}

func (p *Processor) passFilter(ctx context.Context, attrs map[string]string, event *event.Event) bool {
	// Set standard context attributes. The attributes available may not be
	// exactly the same as the attributes defined in the current version of the
	// CloudEvents spec.
	ce := map[string]interface{}{
		"specversion":     event.SpecVersion(),
		"type":            event.Type(),
		"source":          event.Source(),
		"subject":         event.Subject(),
		"id":              event.ID(),
		"time":            event.Time().String(),
		"schemaurl":       event.DataSchema(),
		"datacontenttype": event.DataContentType(),
		"datamediatype":   event.DataMediaType(),
		// TODO: use data_base64 when SDK supports it.
		"datacontentencoding": event.DeprecatedDataContentEncoding(),
	}
	ext := event.Extensions()
	if ext != nil {
		for k, v := range ext {
			ce[k] = v
		}
	}

	for k, v := range attrs {
		var value interface{}
		value, ok := ce[k]
		// If the attribute does not exist in the event, return false.
		if !ok {
			logging.FromContext(ctx).Debug("Attribute not found", zap.String("attribute", k))
			return false
		}
		// If the attribute is not set to any and is different than the one from the event, return false.
		if v != "" && v != value {
			logging.FromContext(ctx).Debug("Attribute had non-matching value", zap.String("attribute", k), zap.String("filter", v), zap.Any("received", value))
			return false
		}
	}
	return true
}
