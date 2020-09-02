// +build wireinject

/*
Copyright 2020 Google LLC.

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

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/broker/ingress"
	"github.com/google/knative-gcp/pkg/metrics"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/wire"
)

func InitializeHandler(
	ctx context.Context,
	port clients.Port,
	projectID clients.ProjectID,
	podName metrics.PodName,
	containerName metrics.ContainerName,
	publishSettings pubsub.PublishSettings,
) (*ingress.Handler, error) {
	panic(wire.Build(
		ingress.HandlerSet,
		wire.Value([]volume.Option(nil)),
		volume.NewTargetsFromFile,
	))
}
