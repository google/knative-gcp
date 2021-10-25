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

	"github.com/google/knative-gcp/pkg/pubsub/adapter"
	"github.com/google/knative-gcp/pkg/utils/clients"

	"github.com/google/wire"
)

func InitializeAdapter(
	ctx context.Context,
	maxConnsPerHost clients.MaxConnsPerHost,
	projectID clients.ProjectID,
	subscriptionID adapter.SubscriptionID,
	namespace adapter.Namespace,
	name adapter.Name,
	resourceGroup adapter.ResourceGroup,
	args *adapter.AdapterArgs) (*adapter.Adapter, error) {
	panic(wire.Build(
		adapter.AdapterSet,
	))
}
