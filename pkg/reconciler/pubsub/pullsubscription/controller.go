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
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/resolver"
	tracingconfig "knative.dev/pkg/tracing/config"

	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "PullSubscriptions"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-pubsub-pullsubscription-controller"

	// defaultResyncPeriod sets the period between reconciliations in case nothing we are watching within the
	// cluster changed. This is needed because a Pub/Sub subscription could have been deleted outside the cluster,
	// and we want to properly update our PullSubscription status.
	defaultResyncPeriod = 5 * time.Minute
)

type envConfig struct {
	// ReceiveAdapter is the receive adapters image. Required.
	ReceiveAdapter string `envconfig:"PUBSUB_RA_IMAGE" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	deploymentInformer := deploymentinformer.Get(ctx)
	pullSubscriptionInformer := pullsubscriptioninformers.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName).Desugar()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	pubsubBase := &pubsub.PubSubBase{
		Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
	}

	r := &Reconciler{
		PubSubBase:             pubsubBase,
		deploymentLister:       deploymentInformer.Lister(),
		pullSubscriptionLister: pullSubscriptionInformer.Lister(),
		receiveAdapterImage:    env.ReceiveAdapter,
		createClientFn:         gpubsub.NewClient,
	}

	impl := controller.NewImpl(r, pubsubBase.Logger, reconcilerName)

	pubsubBase.Logger.Info("Setting up event handlers")
	pullSubscriptionInformer.Informer().AddEventHandlerWithResyncPeriod(controller.HandleAll(impl.Enqueue), defaultResyncPeriod)

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("PullSubscription")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	cmw.Watch(logging.ConfigMapName(), r.UpdateFromLoggingConfigMap)
	cmw.Watch(metrics.ConfigMapName(), r.UpdateFromMetricsConfigMap)
	cmw.Watch(tracingconfig.ConfigName, r.UpdateFromTracingConfigMap)

	return impl
}
