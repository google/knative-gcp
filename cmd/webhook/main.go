/*
Copyright 2019 The Knative Authors

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
	"fmt"
	"golang.org/x/sync/errgroup"
	"knative.dev/pkg/profiling"
	"log"
	"net/http"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"
	"knative.dev/pkg/webhook"

	eventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

const (
	component = "webhook"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

func SharedMain(handlers map[schema.GroupVersionKind]webhook.GenericCRD) {

	flag.Parse()
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatal("Error loading logging configuration:", err)
	}
	config, err := logging.NewConfigFromMap(cm)
	if err != nil {
		log.Fatal("Error parsing logging configuration:", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, component)
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, component))

	logger.Info("Starting the Configuration Webhook")

	// Set up signals so we handle the first shutdown signal gracefully.
	ctx := signals.NewContext()

	clusterConfig, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalw("Failed to get cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatalw("Failed to get the client set", zap.Error(err))
	}

	if err := version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
		logger.Fatalw("Version check failed", err)
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))

	// // If you want to control Defaulting or Validation, you can attach config state
	// // to the context by watching the configmap here, and then uncommenting the logic
	// // below.
	// stores := make([]Store, 0, len(factories))
	// for _, sf := range factories {
	// 	store := sf(logger)
	// 	store.WatchConfigs(configMapWatcher)
	// 	stores = append(stores, store)
	// }

	if err = configMapWatcher.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start the ConfigMap watcher", zap.Error(err))
	}

	options := webhook.ControllerOptions{
		ServiceName:    "webhook",
		DeploymentName: "webhook",
		Namespace:      system.Namespace(),
		Port:           8443,
		SecretName:     "webhook-certs",
		WebhookName:    fmt.Sprintf("webhook.%s.knative.dev", system.Namespace()),
	}

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	controller, err := webhook.NewAdmissionController(kubeClient, options, handlers, logger, ctxFunc, true)

	if err != nil {
		logger.Fatalw("Failed to create admission controller", zap.Error(err))
	}

	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return controller.Run(ctx.Done())
	})
	eg.Go(profilingServer.ListenAndServe)

	// This will block until either a signal arrives or one of the grouped functions
	// returns an error.
	<-egCtx.Done()

	profilingServer.Shutdown(context.Background())
	// Don't forward ErrServerClosed as that indicates we're already shutting down.
	if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
		logger.Errorw("Error while running server", zap.Error(err))
	}
}

func main() {
	handlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"):       &messagingv1alpha1.Channel{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Decorator"):     &messagingv1alpha1.Decorator{},
		eventsv1alpha1.SchemeGroupVersion.WithKind("Storage"):          &eventsv1alpha1.Storage{},
		pubsubv1alpha1.SchemeGroupVersion.WithKind("PullSubscription"): &pubsubv1alpha1.PullSubscription{},
		pubsubv1alpha1.SchemeGroupVersion.WithKind("Topic"):            &pubsubv1alpha1.Topic{},
	}
	SharedMain(handlers)
}
