// Package observability contains common setup and utilities for metrics, logging, and tracing.
package observability

import (
	"context"
	"os"

	"go.uber.org/zap"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	tracingconfig "knative.dev/pkg/tracing/config"
)

// SetupDynamicConfig sets up logging, metrics, and tracing by watching observability
// configmaps. Returns an updated context with logging and function to flush telemetry which should
// be called before exit.
func SetupDynamicConfig(ctx context.Context, componentName string) (context.Context, func(), error) {
	sharedmain.MemStatsOrDie(ctx)
	// Set up our logger.
	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, componentName)
	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := sharedmain.SetupConfigMapWatchOrDie(ctx, logger)
	// Watch the logging config map and dynamically update logging levels.
	sharedmain.WatchLoggingConfigOrDie(ctx, configMapWatcher, logger, atomicLevel, componentName)
	// Watch the observability config map
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap(componentName, logger))
	// Watch the tracing config map
	if err := tracing.SetupDynamicPublishing(logger, configMapWatcher, componentName, tracingconfig.ConfigName); err != nil {
		return ctx, nil, err
	}

	configmapCtx, cancel := context.WithCancel(ctx)
	// configMapWatcher does not block, so start it first.
	if err := configMapWatcher.Start(configmapCtx.Done()); err != nil {
		logger.Warn("Failed to start ConfigMap watcher", zap.Error(err))
	}
	return logging.WithLogger(ctx, logger), func() { cancel(); flushExporters(logger) }, nil
}

// TODO: flush tracers
func flushExporters(logger *zap.SugaredLogger) {
	logger.Sync()
	metrics.FlushExporter()
	os.Stdout.Sync()
	os.Stderr.Sync()
}
