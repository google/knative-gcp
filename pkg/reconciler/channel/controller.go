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

package channel

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"

	deploymentinformer "github.com/knative/pkg/injection/informers/kubeinformers/appsv1/deployment"
	jobinformer "github.com/knative/pkg/injection/informers/kubeinformers/batchv1/job"

	channelinformers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/injection/informers/pubsub/v1alpha1/channel"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-pubsub-source-controller"
)

type envConfig struct {
	// Invoker is the invokers image. Required.
	Invoker string `envconfig:"PUBSUB_INVOKER_IMAGE" required:"true"`

	// TopicOps is the image for operating on topics. Required.
	TopicOps string `envconfig:"PUBSUB_TOPIC_IMAGE" required:"true"`

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
	channelInformer := channelinformers.Get(ctx)
	jobInformer := jobinformer.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	pubsubBase := &pubsub.PubSubBase{
		Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
		TopicOpsImage:        env.TopicOps,
		SubscriptionOpsImage: env.SubscriptionOps,
	}

	c := &Reconciler{
		PubSubBase:       pubsubBase,
		deploymentLister: deploymentInformer.Lister(),
		channelLister:    channelInformer.Lister(),
		invokerImage:     env.Invoker,
	}
	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Channel")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	jobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Channel")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	return impl
}
