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
	"flag"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"

	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	"github.com/google/knative-gcp/pkg/broker/handler/pool/retry"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
)

const (
	component = "broker-retry"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type envConfig struct {
	PodName            string        `envconfig:"POD_NAME" required:"true"`
	ProjectID          string        `envconfig:"PROJECT_ID"`
	TargetsConfigPath  string        `envconfig:"TARGETS_CONFIG_PATH" default:"/var/run/cloud-run-events/broker/targets"`
	HandlerConcurrency int           `envconfig:"HANDLER_CONCURRENCY"`
	TimeoutPerEvent    time.Duration `envconfig:"TIMEOUT_PER_EVENT"`
}

func main() {
	appcredentials.MustExistOrUnsetEnv()
	flag.Parse()

	ctx := signals.NewContext()

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}
	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var: %v", err)
	}

	kubeClient := kubeclient.Get(ctx)

	// We sometimes startup faster than we can reach kube-api. Poll on failure to prevent us terminating.
	if perr := wait.PollImmediate(time.Second, 60*time.Second, func() (bool, error) {
		if err = version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
			log.Printf("Failed to get k8s version %v", err)
		}
		return err == nil, nil
	}); perr != nil {
		log.Fatalf("Timed out attempting to get k8s version: %v", err)
	}

	loggingConfig, err := sharedmain.GetLoggingConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading/parsing logging configuration: %v", err)
	}

	sugaredLogger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
	defer flush(sugaredLogger)
	ctx = logging.WithLogger(ctx, sugaredLogger)

	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	// Watch the observability config map.
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.ConfigMapWatcher(component, nil, sugaredLogger))
	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(sugaredLogger, atomicLevel, component))

	logger := sugaredLogger.Desugar()

	// configMapWatcher does not block, so start it first.
	if err = configMapWatcher.Start(ctx.Done()); err != nil {
		logger.Warn("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatal("Failed to start informers", zap.Error(err))
	}

	// Give the signal channel some buffer so that reconciling handlers won't
	// block the targets config update?
	targetsUpdateCh := make(chan struct{})
	targetsConifg, err := volume.NewTargetsFromFile(
		volume.WithPath(env.TargetsConfigPath),
		volume.WithNotifyChan(targetsUpdateCh))
	if err != nil {
		logger.Fatal("Failed to load targets config", zap.String("path", env.TargetsConfigPath), zap.Error(err))
	}

	logger.Info("Starting the broker retry")

	syncSignal := poolSyncSignal(ctx, targetsUpdateCh)
	syncPool, err := retry.NewSyncPool(targetsConifg, buildPoolOptions(env)...)
	if err != nil {
		logger.Fatal("Failed to get retry sync pool", zap.Error(err))
	}
	if _, err := pool.StartSyncPool(ctx, syncPool, syncSignal); err != nil {
		logger.Fatal("Failed to start retry sync pool", zap.Error(err))
	}

	// Context will be done if a TERM signal is issued.
	<-ctx.Done()
	logger.Info("Exiting...")
}

func poolSyncSignal(ctx context.Context, targetsUpdateCh chan struct{}) chan struct{} {
	// Give it some buffer so that multiple signal could queue up
	// but not blocking the signaler?
	ch := make(chan struct{}, 10)
	ticker := time.NewTicker(15 * time.Second)
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

func buildPoolOptions(env envConfig) []pool.Option {
	rs := pubsub.DefaultReceiveSettings
	// If Synchronous is true, then no more than MaxOutstandingMessages will be in memory at one time.
	// MaxOutstandingBytes still refers to the total bytes processed, rather than in memory.
	// NumGoroutines is ignored.
	// TODO Need to revisit it. For the case when synchronous is true, default value of MaxOutstandingMessages and MaxOutstandingBytes might need to override.
	rs.Synchronous = true
	var opts []pool.Option
	if env.HandlerConcurrency > 0 {
		opts = append(opts, pool.WithHandlerConcurrency(env.HandlerConcurrency))
		rs.NumGoroutines = env.HandlerConcurrency
	}
	if env.ProjectID != "" {
		opts = append(opts, pool.WithProjectID(env.ProjectID))
	}
	if env.TimeoutPerEvent > 0 {
		opts = append(opts, pool.WithTimeoutPerEvent(env.TimeoutPerEvent))
	}
	opts = append(opts, pool.WithPubsubReceiveSettings(rs))
	// The default CeClient is good?
	return opts
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
