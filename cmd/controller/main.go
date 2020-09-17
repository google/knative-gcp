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
	"log"

	"google.golang.org/api/option"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/google/knative-gcp/pkg/reconciler/broker"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell"
	"github.com/google/knative-gcp/pkg/reconciler/deployment"
	"github.com/google/knative-gcp/pkg/reconciler/events/auditlogs"
	"github.com/google/knative-gcp/pkg/reconciler/events/build"
	"github.com/google/knative-gcp/pkg/reconciler/events/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/events/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler/events/storage"
	kedapullsubscription "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/keda"
	staticpullsubscription "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/static"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/topic"
	"github.com/google/knative-gcp/pkg/reconciler/messaging/channel"
	"github.com/google/knative-gcp/pkg/reconciler/trigger"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {
	appcredentials.MustExistOrUnsetEnv()
	ctx := signals.NewContext()
	controllers, err := InitializeControllers(ctx)
	if err != nil {
		log.Fatal(err)
	}
	sharedmain.MainWithContext(ctx, "controller", controllers...)
}

func Controllers(
	auditlogsController auditlogs.Constructor,
	storageController storage.Constructor,
	schedulerController scheduler.Constructor,
	pubsubController pubsub.Constructor,
	buildController build.Constructor,
	pullsubscriptionController staticpullsubscription.Constructor,
	kedaPullsubscriptionController kedapullsubscription.Constructor,
	topicController topic.Constructor,
	channelController channel.Constructor,
	triggerController trigger.Constructor,
	brokerController broker.Constructor,
	deploymentController deployment.Constructor,
	brokercellController brokercell.Constructor,
) []injection.ControllerConstructor {
	return []injection.ControllerConstructor{
		injection.ControllerConstructor(auditlogsController),
		injection.ControllerConstructor(storageController),
		injection.ControllerConstructor(schedulerController),
		injection.ControllerConstructor(pubsubController),
		injection.ControllerConstructor(buildController),
		injection.ControllerConstructor(pullsubscriptionController),
		injection.ControllerConstructor(kedaPullsubscriptionController),
		injection.ControllerConstructor(topicController),
		injection.ControllerConstructor(channelController),
		injection.ControllerConstructor(triggerController),
		injection.ControllerConstructor(brokerController),
		injection.ControllerConstructor(deploymentController),
		injection.ControllerConstructor(brokercellController),
	}
}

func ClientOptions() []option.ClientOption {
	return nil
}
