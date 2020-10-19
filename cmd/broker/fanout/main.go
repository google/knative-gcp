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

package main

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub"

	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/broker/handler"
	"github.com/google/knative-gcp/pkg/metrics"
	"github.com/google/knative-gcp/pkg/utils"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/knative-gcp/pkg/utils/mainhelper"

	"go.uber.org/zap"
)

const (
	component        = "broker-fanout"
	metricNamespace  = "trigger"
	poolResyncPeriod = 15 * time.Second
)

type envConfig struct {
	PodName                string `envconfig:"POD_NAME" required:"true"`
	TargetsConfigPath      string `envconfig:"TARGETS_CONFIG_PATH" default:"/var/run/cloud-run-events/broker/targets"`
	HandlerConcurrency     int    `envconfig:"HANDLER_CONCURRENCY"`
	MaxConcurrencyPerEvent int    `envconfig:"MAX_CONCURRENCY_PER_EVENT"`

	// MaxStaleDuration is the max duration of the handler pool without being synced.
	// With the internal pool resync period being 15s, it requires at least 4
	// continuous sync failures (or no sync at all) to be stale.
	MaxStaleDuration time.Duration `envconfig:"MAX_STALE_DURATION" default:"1m"`

	// MaxOutstandingBytes is the maximum size of unprocessed messages (unacknowledged but not yet expired).
	// Default is 400Mb
	MaxOutstandingBytes int `envconfig:"MAX_OUTSTANDING_BYTES" default:"400000000"`

	// MaxOutstandingMessages is the maximum number of unprocessed messages (unacknowledged but not yet expired).
	MaxOutstandingMessages int `envconfig:"MAX_OUTSTANDING_MESSAGES" default:"4000"`

	// Max to 10m.
	TimeoutPerEvent time.Duration `envconfig:"TIMEOUT_PER_EVENT"`
}

func main() {
	appcredentials.MustExistOrUnsetEnv()

	var env envConfig
	ctx, res := mainhelper.Init(component, mainhelper.WithMetricNamespace(metricNamespace), mainhelper.WithEnv(&env))
	defer res.Cleanup()
	logger := res.Logger

	if env.MaxStaleDuration > 0 && env.MaxStaleDuration < poolResyncPeriod {
		logger.Fatalf("MAX_STALE_DURATION must be greater than pool resync period %v", poolResyncPeriod)
	}

	// Give the signal channel some buffer so that reconciling handlers won't
	// block the targets config update?
	targetsUpdateCh := make(chan struct{})

	logger.Info("Starting the broker fanout")

	projectID, err := utils.ProjectIDOrDefault("")
	if err != nil {
		logger.Fatalf("failed to get default ProjectID: %v", err)
	}

	syncSignal := poolSyncSignal(ctx, targetsUpdateCh)
	syncPool, err := InitializeSyncPool(
		ctx,
		clients.ProjectID(projectID),
		metrics.PodName(env.PodName),
		metrics.ContainerName(component),
		[]volume.Option{
			volume.WithPath(env.TargetsConfigPath),
			volume.WithNotifyChan(targetsUpdateCh),
		},
		buildHandlerOptions(env)...,
	)
	if err != nil {
		logger.Fatal("Failed to create fanout sync pool", zap.Error(err))
	}
	if _, err := handler.StartSyncPool(ctx, syncPool, syncSignal, env.MaxStaleDuration, handler.DefaultHealthCheckPort); err != nil {
		logger.Fatalw("Failed to start fanout sync pool", zap.Error(err))
	}

	// Context will be done if a TERM signal is issued.
	<-ctx.Done()
	// Wait a grace period for the handlers to shutdown.
	time.Sleep(30 * time.Second)
	logger.Info("Done waiting, exit.")
}

func poolSyncSignal(ctx context.Context, targetsUpdateCh chan struct{}) chan struct{} {
	// Give it some buffer so that multiple signal could queue up
	// but not blocking the signaler?
	ch := make(chan struct{}, 10)
	ticker := time.NewTicker(poolResyncPeriod)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-targetsUpdateCh:
				ch <- struct{}{}
			case <-ticker.C:
				ch <- struct{}{}
			}
		}
	}()
	return ch
}

func buildHandlerOptions(env envConfig) []handler.Option {
	rs := pubsub.DefaultReceiveSettings
	var opts []handler.Option
	if env.HandlerConcurrency > 0 {
		// Let the pubsub subscription and handler have the same concurrency?
		opts = append(opts, handler.WithHandlerConcurrency(env.HandlerConcurrency))
		rs.NumGoroutines = env.HandlerConcurrency
	}
	if env.MaxConcurrencyPerEvent > 0 {
		opts = append(opts, handler.WithMaxConcurrentPerEvent(env.MaxConcurrencyPerEvent))
	}
	if env.TimeoutPerEvent > 0 {
		opts = append(opts, handler.WithTimeoutPerEvent(env.TimeoutPerEvent))
	}
	if env.MaxOutstandingBytes > 0 {
		rs.MaxOutstandingBytes = env.MaxOutstandingBytes
	}
	if env.MaxOutstandingMessages > 0 {
		rs.MaxOutstandingMessages = env.MaxOutstandingMessages
	}
	opts = append(opts, handler.WithPubsubReceiveSettings(rs))
	// The default CeClient is good?
	return opts
}
