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

package pubsubsource

import (
	"context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/kelseyhightower/envconfig"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	pubsubsourceinformers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/injection/informers/events/v1alpha1/pubsubsource"
	deploymentinformer "github.com/knative/pkg/injection/informers/kubeinformers/appsv1/deployment"
	jobinformer "github.com/knative/pkg/injection/informers/kubeinformers/batchv1/job"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-pubsub-source-controller"
)

type envConfig struct {
	// ReceiveAdapter is the receive adapters image. Required.
	ReceiveAdapter string `envconfig:"PUBSUB_RA_IMAGE" required:"true"`

	// SubscriptionOps is the image for operating on subscriptions. Required.
	SubscriptionOps string `envconfig:"PUBSUB_SUB_IMAGE" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	deploymentInformer := deploymentinformer.Get(ctx)
	sourceInformer := pubsubsourceinformers.Get(ctx)
	jobInformer := jobinformer.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	c := &Reconciler{
		Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
		deploymentLister:     deploymentInformer.Lister(),
		sourceLister:         sourceInformer.Lister(),
		receiveAdapterImage:  env.ReceiveAdapter,
		subscriptionOpsImage: env.SubscriptionOps,
	}
	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers")
	sourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("PubSubSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	jobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("PubSubSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	return impl
}
