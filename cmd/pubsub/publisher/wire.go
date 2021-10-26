//go:build wireinject
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

	"github.com/google/knative-gcp/pkg/pubsub/publisher"
	"github.com/google/knative-gcp/pkg/utils/authcheck"
	"github.com/google/knative-gcp/pkg/utils/clients"

	"github.com/google/wire"
)

func InitializePublisher(
	ctx context.Context,
	port clients.Port,
	projectID clients.ProjectID,
	topicID publisher.TopicID,
	authType authcheck.AuthType,
) (*publisher.Publisher, error) {
	panic(wire.Build(
		publisher.PublisherSet,
	))
}
