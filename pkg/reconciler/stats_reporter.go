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

package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/knative/pkg/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

type Measurement int

const (
	// ChannelReadyCountN is the number of channels that have become ready.
	ChannelReadyCountN = "channel_ready_count"
	// ChannelReadyLatencyN is the time it takes for a channel to become ready since the resource is created.
	ChannelReadyLatencyN = "channel_ready_latency"

	// PullSubscriptionReadyCountN is the number of pull subscriptions that have become ready.
	PullSubscriptionReadyCountN = "pullsubscription_ready_count"
	// PullSubscriptionReadyLatencyN is the time it takes for a pull subscription to become ready since the resource is created.
	PullSubscriptionReadyLatencyN = "pullsubscription_ready_latency"
)

var (
	KindToStatKeys = map[string]StatKey{
		"Channel": {
			ReadyCountKey:   ChannelReadyCountN,
			ReadyLatencyKey: ChannelReadyLatencyN,
		},
		"PullSubscription": {
			ReadyCountKey:   PullSubscriptionReadyCountN,
			ReadyLatencyKey: PullSubscriptionReadyLatencyN,
		},
	}

	KindToMeasurements map[string]Measurements

	reconcilerTagKey tag.Key
	keyTagKey        tag.Key
)

type Measurements struct {
	ReadyLatencyStat *stats.Int64Measure
	ReadyCountStat   *stats.Int64Measure
}

type StatKey struct {
	ReadyLatencyKey string
	ReadyCountKey   string
}

func init() {
	var err error
	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	reconcilerTagKey = mustNewTagKey("reconciler")
	keyTagKey = mustNewTagKey("key")

	KindToMeasurements = make(map[string]Measurements, len(KindToStatKeys))

	for kind, keys := range KindToStatKeys {

		readyLatencyStat := stats.Int64(
			keys.ReadyLatencyKey,
			fmt.Sprintf("Time it takes for a %s to become ready since created", kind),
			stats.UnitMilliseconds)

		readyCountStat := stats.Int64(
			keys.ReadyCountKey,
			fmt.Sprintf("Number of %s that became ready", kind),
			stats.UnitDimensionless)

		// Save the measurements for later marks.
		KindToMeasurements[kind] = Measurements{
			ReadyCountStat:   readyCountStat,
			ReadyLatencyStat: readyLatencyStat,
		}

		// Create views to see our measurements. This can return an error if
		// a previously-registered view has the same name with a different value.
		// View name defaults to the measure name if unspecified.
		err = view.Register(
			&view.View{
				Description: readyLatencyStat.Description(),
				Measure:     readyLatencyStat,
				Aggregation: view.LastValue(),
				TagKeys:     []tag.Key{reconcilerTagKey, keyTagKey},
			},
			&view.View{
				Description: readyCountStat.Description(),
				Measure:     readyCountStat,
				Aggregation: view.Count(),
				TagKeys:     []tag.Key{reconcilerTagKey, keyTagKey},
			},
		)
		if err != nil {
			panic(err)
		}
	}
}

// StatsReporter reports reconcilers' metrics.
type StatsReporter interface {
	// ReportReady reports the time it took a resource to become Ready.
	ReportReady(kind, namespace, service string, d time.Duration) error
}

type reporter struct {
	ctx context.Context
}

// NewStatsReporter creates a reporter for reconcilers' metrics
func NewStatsReporter(reconciler string) (StatsReporter, error) {
	ctx, err := tag.New(
		context.Background(),
		tag.Insert(reconcilerTagKey, reconciler))
	if err != nil {
		return nil, err
	}
	return &reporter{ctx: ctx}, nil
}

// srKey is used to associate StatsReporters with contexts.
type srKey struct{}

// WithStatsReporter attaches the given StatsReporter to the provided context
// in the returned context.
func WithStatsReporter(ctx context.Context, sr StatsReporter) context.Context {
	return context.WithValue(ctx, srKey{}, sr)
}

// GetStatsReporter attempts to look up the StatsReporter on a given context.
// It may return null if none is found.
func GetStatsReporter(ctx context.Context) StatsReporter {
	untyped := ctx.Value(srKey{})
	if untyped == nil {
		return nil
	}
	return untyped.(StatsReporter)
}

// ReportServiceReady reports the time it took a service to become Ready
func (r *reporter) ReportReady(kind, namespace, service string, d time.Duration) error {
	key := fmt.Sprintf("%s/%s", namespace, service)
	v := int64(d / time.Millisecond)
	ctx, err := tag.New(
		r.ctx,
		tag.Insert(keyTagKey, key))
	if err != nil {
		return err
	}

	m, ok := KindToMeasurements[kind]
	if !ok {
		return fmt.Errorf("unknown kind attempted to report ready, %q", kind)
	}

	metrics.Record(ctx, m.ReadyCountStat.M(1))
	metrics.Record(ctx, m.ReadyLatencyStat.M(v))
	return nil
}

func mustNewTagKey(s string) tag.Key {
	tagKey, err := tag.NewKey(s)
	if err != nil {
		panic(err)
	}
	return tagKey
}
