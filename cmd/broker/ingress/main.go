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
	"flag"
	"github.com/google/knative-gcp/pkg/metrics"
	"log"

	"github.com/google/knative-gcp/pkg/broker/ingress"
	"github.com/google/knative-gcp/pkg/observability"
	"github.com/google/knative-gcp/pkg/utils"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

const containerName = "ingress"

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type envConfig struct {
	PodName   string `envconfig:"POD_NAME" required:"true"`
	Port      int    `envconfig:"PORT" default:"8080"`
	ProjectID string `envconfig:"PROJECT_ID"`
}

const (
	componentName = "broker"
)

// main creates and starts an ingress handler using default options.
// 1. It listens on port specified by "PORT" env var, or default 8080 if env var is not set
// 2. It reads "PROJECT_ID" env var for pubsub project. If the env var is empty, it retrieves project ID from
//    GCE metadata.
// 3. It expects broker configmap mounted at "/var/run/cloud-run-events/broker/targets"
func main() {
	appcredentials.MustExistOrUnsetEnv()

	ctx := signals.NewContext()

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, _ = injection.Default.SetupInformers(ctx, cfg)

	ctx, flush, err := observability.SetupDynamicConfig(ctx, componentName)
	if err != nil {
		log.Fatal("Error setting up dynamic observability configuration", err)
	}
	defer flush()
	logger := logging.FromContext(ctx)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Desugar().Fatal("Failed to process env var", zap.Error(err))
	}
	projectID, err := utils.ProjectID(env.ProjectID)
	if err != nil {
		logger.Desugar().Fatal("Failed to create project id", zap.Error(err))
	}
	logger.Desugar().Info("Starting ingress handler", zap.Any("envConfig", env), zap.Any("Project ID", projectID))

	ingress, err := InitializeHandler(
		ctx,
		ingress.Port(env.Port),
		ingress.ProjectID(projectID),
		metrics.PodName(env.PodName),
		metrics.ContainerName(containerName),
	)
	if err != nil {
		logger.Desugar().Fatal("Unable to create ingress handler: ", zap.Error(err))
	}

	logger.Desugar().Info("Starting ingress.", zap.Any("ingress", ingress))
	if err := ingress.Start(ctx); err != nil {
		logger.Desugar().Fatal("failed to start ingress: ", zap.Error(err))
	}
}
