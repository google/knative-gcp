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
	"os"
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
	pkglogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"

	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	"github.com/google/knative-gcp/pkg/broker/handler/pool/fanout"
)

const (
	component = "broker-fanout"
)

var (
	masterURL = flag.String("master", "", "The address of the Kubernetes API server. "+
		"Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type envConfig struct {
	PodName                string        `envconfig:"POD_NAME" required:"true"`
	ProjectID              string        `envconfig:"PROJECT_ID"`
	TargetsConfigPath      string        `envconfig:"TARGETS_CONFIG_PATH"`
	HandlerConcurrency     int           `envconfig:"HANDLER_CONCURRENCY"`
	MaxConcurrencyPerEvent int           `envconfig:"MAX_CONCURRENCY_PER_EVENT"`
	TimeoutPerEvent        time.Duration `envconfig:"TIMEOUT_PER_EVENT"`
}

func main() {
	flag.Parse()

	// Set up a context that we can cancel to tell informers and other subprocesses to stop.
	ctx := signals.NewContext()

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig:", err)
	}

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", err)
	}

	kubeClient := kubeclient.Get(ctx)

	// We sometimes startup faster than we can reach kube-api. Poll on failure to prevent us terminating
	if perr := wait.PollImmediate(time.Second, 60*time.Second, func() (bool, error) {
		if err = version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
			log.Printf("Failed to get k8s version %v", err)
		}
		return err == nil, nil
	}); perr != nil {
		log.Fatal("Timed out attempting to get k8s version: ", err)
	}

	// Set up our logger.
	loggingConfig, err := sharedmain.GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration: ", err)
	}

	logger, atomicLevel := pkglogging.NewLoggerFromConfig(loggingConfig, component)
	defer flush(logger)
	ctx = pkglogging.WithLogger(ctx, logger)

	// Run informers instead of starting them from the factory to prevent the sync hanging because of empty handler.
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatalw("Failed to start informers", zap.Error(err))
	}

	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher.Watch(pkglogging.ConfigMapName(), pkglogging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
	// Watch the observability config map
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap(component, logger))

	// Give the signal channel some buffer so that reconciling handlers won't
	// block the targets config update?
	targetsUpdateCh := make(chan struct{})
	targetsConifg, err := volume.NewTargetsFromFile(
		volume.WithPath(env.TargetsConfigPath),
		volume.WithNotifyChan(targetsUpdateCh))
	if err != nil {
		logger.Fatalw("Failed to load targets config", zap.String("path", env.TargetsConfigPath), zap.Error(err))
	}

	logger.Info("Starting the broker fanout")

	syncSignal := poolSyncSignal(ctx, targetsUpdateCh)
	if _, err := fanout.StartSyncPool(ctx, targetsConifg, buildPoolOptions(env, syncSignal)...); err != nil {
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

func buildPoolOptions(env envConfig, syncSignal chan struct{}) []pool.Option {
	rs := pubsub.DefaultReceiveSettings
	var opts []pool.Option
	if env.HandlerConcurrency > 0 {
		// Let the pubsub subscription and handler have the same concurrency?
		opts = append(opts, pool.WithHandlerConcurrency(env.HandlerConcurrency))
		rs.NumGoroutines = env.HandlerConcurrency
	}
	if env.MaxConcurrencyPerEvent > 0 {
		opts = append(opts, pool.WithMaxConcurrentPerEvent(env.MaxConcurrencyPerEvent))
	}
	if env.ProjectID != "" {
		opts = append(opts, pool.WithProjectID(env.ProjectID))
	}
	if env.TimeoutPerEvent > 0 {
		opts = append(opts, pool.WithTimeoutPerEvent(env.TimeoutPerEvent))
	}
	opts = append(opts, pool.WithPubsubReceiveSettings(rs))
	opts = append(opts, pool.WithSyncSignal(syncSignal))
	// The default CeClient is good?
	return opts
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	os.Stdout.Sync()
	os.Stderr.Sync()
	metrics.FlushExporter()
}
