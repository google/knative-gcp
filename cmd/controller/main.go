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

package main

import (
	"flag"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"log"
	"os"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	informers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/informers/externalversions"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsubsource"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/metrics"
	pkgmetrics "github.com/knative/pkg/metrics"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
)

const (
	component = "controller"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raPubSubImageEnvVar = "GCPPUBSUB_RA_IMAGE"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()

	// Set up our logger.
	loggingConfigMap, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatal("Error loading logging configuration:", err)
	}
	loggingConfig, err := logging.NewConfigFromMap(loggingConfigMap)
	if err != nil {
		log.Fatal("Error parsing logging configuration:", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
	defer flush(logger)

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalw("Error building kubeconfig", zap.Error(err))
	}

	const numControllers = 1
	cfg.QPS = numControllers * rest.DefaultQPS
	cfg.Burst = numControllers * rest.DefaultBurst
	opt := reconciler.NewOptionsOrDie(cfg, logger, stopCh)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(opt.KubeClientSet, opt.ResyncPeriod)
	runInformerFactory := informers.NewSharedInformerFactory(opt.RunClientSet, opt.ResyncPeriod)

	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	pubsubSourceInformer := runInformerFactory.Events().V1alpha1().PubSubSources()

	raPubSubSourceImage, defined := os.LookupEnv(raPubSubImageEnvVar)
	if !defined {
		logger.Fatalw("required environment variable '%s' not defined", raPubSubImageEnvVar)
	}

	// Build all of our controllers, with the clients constructed above.
	// Add new controllers to this array.
	controllers := [...]*controller.Impl{
		pubsubsource.NewController(
			opt,
			deploymentInformer,
			pubsubSourceInformer,
			raPubSubSourceImage,
		),
	}
	// This line asserts at compile time that the length of controllers is equal to numControllers.
	// It is based on https://go101.org/article/tips.html#assert-at-compile-time, which notes that
	// var _ [N-M]int
	// asserts at compile time that N >= M, which we can use to establish equality of N and M:
	// (N >= M) && (M >= N) => (N == M)
	var _ [numControllers - len(controllers)][len(controllers) - numControllers]int

	// Watch the logging config map and dynamically update logging levels.
	opt.ConfigMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
	// Watch the observability config map and dynamically update metrics exporter.
	opt.ConfigMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap(component, logger))
	if err := opt.ConfigMapWatcher.Start(stopCh); err != nil {
		logger.Fatalw("failed to start configuration manager", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(
		stopCh,
		deploymentInformer.Informer(),
		pubsubSourceInformer.Informer(),
	); err != nil {
		logger.Fatalw("Failed to start informers", err)
	}

	// Start all of the controllers.
	logger.Info("Starting controllers.")
	controller.StartAll(stopCh, controllers[:]...)
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	pkgmetrics.FlushExporter()
}
