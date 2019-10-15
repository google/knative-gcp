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

package adapter

// TODO refactor existing stats_reporter metrics stuff and see if we can use this instead.
//  Also add more interesting metrics.

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// LatencyMs measures the latency in milliseconds for the PullSubscription
	// adapter methods for Pub/Sub.
	LatencyMs = stats.Float64(
		"events.cloud.google.com/pubsub/adapter/latency",
		"The latency in milliseconds for the PullSubscription adapter methods for Pub/Sub.",
		"ms")
)

var (
	// LatencyView is an OpenCensus view that shows http transport method latency.
	LatencyView = &view.View{
		Name:        "pubsub/pullsubscriptions/adapter/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of PullSubscription adapter for Pub/Sub.",
		// Bucket boundaries are 10ms, 100ms, 1s, 10s, 30s and 60s.
		Aggregation: view.Distribution(10, 100, 1000, 10000, 30000, 60000),
		TagKeys:     observability.LatencyTags(),
	}
)

type observed int32

// Adheres to Observable
var _ observability.Observable = observed(0)

const (
	reportNewPubSubClient observed = iota
	reportNewHTTPClient
	reportReceive
	reportConvert
)

// TraceName implements Observable.TraceName
func (o observed) TraceName() string {
	switch o {
	case reportNewPubSubClient:
		return "pubsub/pullsubscriptions/adapter/client/pubsub/new"
	case reportNewHTTPClient:
		return "pubsub/pullsubscriptions/adapter/client/http/new"
	case reportReceive:
		return "pubsub/pullsubscriptions/adapter/receive"
	case reportConvert:
		return "pubsub/pullsubscriptions/adapter/receive/convert"
	default:
		return "pubsub/pullsubscriptions/adapter/unknown"
	}
}

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportNewPubSubClient:
		return "newPubSubClient"
	case reportNewHTTPClient:
		return "newHTTPClient"
	case reportReceive:
		return "receive"
	case reportConvert:
		return "convert"
	default:
		return "unknown"
	}
}

// LatencyMs implements Observable.LatencyMs
func (o observed) LatencyMs() *stats.Float64Measure {
	return LatencyMs
}

// CodecObserved is a wrapper to append version to observed.
type CodecObserved struct {
	// Method
	o observed
}

// Adheres to Observable
var _ observability.Observable = (*CodecObserved)(nil)

// TraceName implements Observable.TraceName
func (c CodecObserved) TraceName() string {
	return c.o.TraceName()
}

// MethodName implements Observable.MethodName
func (c CodecObserved) MethodName() string {
	return c.o.MethodName()
}

// LatencyMs implements Observable.LatencyMs
func (c CodecObserved) LatencyMs() *stats.Float64Measure {
	return c.o.LatencyMs()
}
