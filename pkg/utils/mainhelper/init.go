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

// Package mainhelper provides helper functions for common boilerplate code in
// writing a main function such as setting up kube informers.
package mainhelper

import (
	"context"
	"log"
	"net/http"

	"github.com/google/knative-gcp/pkg/observability"
	"github.com/google/knative-gcp/pkg/utils/profiling"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	pkgprofiling "knative.dev/pkg/profiling"
)

// InitRes holds a collection of objects after init for convenient access by
// other custom logic in the main function.
type InitRes struct {
	Logger     *zap.SugaredLogger
	KubeClient kubernetes.Interface
	CMPWatcher configmap.Watcher
	Cleanup    func()
}

// Init runs common logic in starting a main function, similar to sharedmain.
// it returns a result object that contains useful artifacts for later use.
// Unlike sharedmain.Main, Init is meant to be run as a helper function in any main
// functions, while sharedmain.Main runs controllers with predefined method signatures.
//
// When a command is converted to use this function, please update the list of
// commands that support profiling in docs/development/profiling.md.
func Init(component string, opts ...InitOption) (context.Context, *InitRes) {
	args := newInitArgs(component, opts...)
	ctx := args.ctx
	ProcessEnvConfigOrDie(args.env)

	log.Printf("Registering %d clients", len(args.injection.GetClients()))
	log.Printf("Registering %d informer factories", len(args.injection.GetInformerFactories()))
	log.Printf("Registering %d informers", len(args.injection.GetInformers()))
	ctx, informers := args.injection.SetupInformers(ctx, args.kubeConfig)

	ctx, cmw, profilingHandler, flush := observability.SetupDynamicConfigOrDie(ctx, component, args.metricNamespace)
	logger := logging.FromContext(ctx)
	RunProfilingServer(ctx, logger, profilingHandler)

	// Try starting the GCP Profiler if config is present.
	ProcessGCPProfilerEnvConfigOrDie(component, logger)

	// This is currently a workaround for testing where k8s APIServer is not properly setup.
	if !args.skipK8sVersionCheck {
		sharedmain.CheckK8sClientMinimumVersionOrDie(ctx, logger)
	}

	logger.Info("Starting informers...")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Desugar().Fatal("Failed to start informers", zap.Error(err))
	}

	return ctx, &InitRes{
		Logger:     logger,
		KubeClient: kubeclient.Get(ctx),
		CMPWatcher: cmw,
		Cleanup:    flush,
	}
}

// ProcessEnvConfigOrDie retrieves environment variables.
func ProcessEnvConfigOrDie(env interface{}) {
	if env == nil {
		return
	}
	if err := envconfig.Process("", env); err != nil {
		log.Fatal("Failed to process env var", err)
	}
	log.Printf("Running with env: %+v", env)
}

// ProcessGCPProfilerEnvConfigOrDie tries to enable the GCP Profiler if env vars
// have configured it.
func ProcessGCPProfilerEnvConfigOrDie(component string, logger *zap.SugaredLogger) {
	var env profiling.GCPProfilerEnvConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Desugar().Fatal("Failed to process GCP Profiler env config", zap.Error(err))
	}
	if env.GCPProfilerEnabled() {
		if err := profiling.StartGCPProfiler(component, env); err != nil {
			logger.Desugar().Fatal("Failed to start GCP Profiler", zap.Error(err))
		}
		logger.Desugar().Info("GCP Profiler enabled", zap.Bool("gcpProfiler", true), zap.Any("gcpProfilerConfig", env))
	}
}

// RunProfilingServer starts a profiling server.
func RunProfilingServer(ctx context.Context, logger *zap.SugaredLogger, h *pkgprofiling.Handler) {
	profilingServer := pkgprofiling.NewServer(h)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(profilingServer.ListenAndServe)
	go func() {
		// This will block until either a signal arrives or one of the grouped functions
		// returns an error.
		<-egCtx.Done()

		profilingServer.Shutdown(context.Background())
		if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
			logger.Error("Error while running server", zap.Error(err))
		}
	}()
}
