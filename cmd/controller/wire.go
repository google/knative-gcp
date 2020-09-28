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

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	"github.com/google/knative-gcp/pkg/reconciler/broker"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell"
	"github.com/google/knative-gcp/pkg/reconciler/deployment"
	"github.com/google/knative-gcp/pkg/reconciler/events/auditlogs"
	"github.com/google/knative-gcp/pkg/reconciler/events/build"
	"github.com/google/knative-gcp/pkg/reconciler/events/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/events/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler/events/storage"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/keda"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/static"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/topic"
	"github.com/google/knative-gcp/pkg/reconciler/messaging/channel"
	"github.com/google/knative-gcp/pkg/reconciler/trigger"
	"github.com/google/wire"
	"knative.dev/pkg/injection"
)

func InitializeControllers(ctx context.Context) ([]injection.ControllerConstructor, error) {
	panic(wire.Build(
		Controllers,
		ClientOptions,
		iam.PolicyManagerSet,
		wire.Struct(new(gcpauth.StoreSingleton)),
		wire.Struct(new(dataresidency.StoreSingleton)),
		auditlogs.NewConstructor,
		storage.NewConstructor,
		scheduler.NewConstructor,
		pubsub.NewConstructor,
		build.NewConstructor,
		static.NewConstructor,
		keda.NewConstructor,
		topic.NewConstructor,
		channel.NewConstructor,
		trigger.NewConstructor,
		broker.NewConstructor,
		deployment.NewConstructor,
		brokercell.NewConstructor,
	))
}
