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

package pullsubscription

import (
	"context"

	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/pkg/metrics"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"

	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	jobinformer "knative.dev/pkg/client/injection/kube/informers/batch/v1/job"

	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-pubsub-pullsubscription-controller"
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
	sourceInformer := pullsubscriptioninformers.Get(ctx)
	jobInformer := jobinformer.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	pubsubBase := &pubsub.PubSubBase{
		Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
		SubscriptionOpsImage: env.SubscriptionOps,
	}

	c := &Reconciler{
		PubSubBase:          pubsubBase,
		deploymentLister:    deploymentInformer.Lister(),
		sourceLister:        sourceInformer.Lister(),
		receiveAdapterImage: env.ReceiveAdapter,
	}
	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers")
	sourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("PullSubscription")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	jobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("PullSubscription")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	cmw.Watch(logging.ConfigMapName(), c.UpdateFromLoggingConfigMap)
	cmw.Watch(metrics.ConfigMapName(), c.UpdateFromMetricsConfigMap)
	cmw.Watch(tracingconfig.ConfigName, c.UpdateFromTracingConfigMap)

	return impl
}
