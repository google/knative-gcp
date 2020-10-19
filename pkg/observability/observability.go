// Package observability contains common setup and utilities for metrics, logging, and tracing.
package observability

import (
	"context"
	"os"

	"go.uber.org/zap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"
)

// SetupDynamicConfigOrDie sets up logging, metrics, and tracing by watching observability
// configmaps. Returns an updated context with logging and function to flush telemetry which should
// be called before exit.
// The input context should have KubeClient injected.
func SetupDynamicConfigOrDie(ctx context.Context, componentName string, metricNamespace string) (context.Context, *configmap.InformedWatcher, *profiling.Handler, func()) {
	metrics.MemStatsOrDie(ctx)
	// Set up our logger.
	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, componentName)
	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := sharedmain.SetupConfigMapWatchOrDie(ctx, logger)
	// Watch the logging config map and dynamically update logging levels.
	sharedmain.WatchLoggingConfigOrDie(ctx, configMapWatcher, logger, atomicLevel, componentName)
	// Watch the observability config map
	ph := profiling.NewHandler(logger, false)
	sharedmain.WatchObservabilityConfigOrDie(ctx, configMapWatcher, ph, logger, metricNamespace)
	// Watch the tracing config map
	setupTracingOrDie(configMapWatcher, logger, componentName)

	configMapCtx, cancel := context.WithCancel(ctx)
	// configMapWatcher does not block, so start it first.
	logger.Info("Starting configuration manager...")
	if err := configMapWatcher.Start(configMapCtx.Done()); err != nil {
		logger.Fatal("Failed to start ConfigMap watcher", zap.Error(err))
	}
	return logging.WithLogger(ctx, logger), configMapWatcher, ph, func() { cancel(); flushExporters(logger) }
}

func setupTracingOrDie(configMapWatcher *configmap.InformedWatcher, logger *zap.SugaredLogger, componentName string) {
	if err := tracing.SetupDynamicPublishing(logger, configMapWatcher, componentName, tracingconfig.ConfigName); err != nil {
		logger.With(zap.Error(err)).Fatalf("Error reading ConfigMap %q", tracingconfig.ConfigName)
	}
}

// TODO: flush tracers
func flushExporters(logger *zap.SugaredLogger) {
	logger.Sync()
	metrics.FlushExporter()
	os.Stdout.Sync()
	os.Stderr.Sync()
}
