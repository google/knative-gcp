// Package observability contains common setup and utilities for metrics, logging, and tracing.
package observability

import (
	"context"
	"log"
	"os"

	"go.uber.org/zap"
	"knative.dev/eventing/pkg/tracing"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/system"
	tracingconfig "knative.dev/pkg/tracing/config"
)

// SetupDynamicConfig sets up logging, metrics, and tracing by watching observability
// configmaps. Returns an updated context with logging and function to flush telemetry which should
// be called before exit.
func SetupDynamicConfig(ctx context.Context, componentName string) (context.Context, func(), error) {
	// Set up our logger.
	loggingConfig, err := sharedmain.GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration: ", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, componentName)
	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, componentName))
	// Watch the observability config map
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap(componentName, logger))
	// Watch the tracing config map
	if err := tracing.SetupDynamicPublishing(logger, configMapWatcher, componentName, tracingconfig.ConfigName); err != nil {
		return ctx, nil, err
	}

	configmapCtx, cancel := context.WithCancel(ctx)
	// configMapWatcher does not block, so start it first.
	if err = configMapWatcher.Start(configmapCtx.Done()); err != nil {
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
