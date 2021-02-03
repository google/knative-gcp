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

package static

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/injection"

	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	serviceaccountinformers "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	tracingconfig "knative.dev/pkg/tracing/config"

	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	"github.com/google/knative-gcp/pkg/apis/duck"
	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1/pullsubscription"
	pullsubscriptionreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1/pullsubscription"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	psreconciler "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription"
	"github.com/google/knative-gcp/pkg/utils/authcheck"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "PullSubscriptions"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "events-system-pubsub-pullsubscription-controller"

	resourceGroup = "pullsubscriptions.internal.events.cloud.google.com"
)

type envConfig struct {
	// ReceiveAdapter is the receive adapters image. Required.
	ReceiveAdapter string `envconfig:"PUBSUB_RA_IMAGE" required:"true"`
}

type Constructor injection.ControllerConstructor

// NewConstructor creates a constructor to make a static pullsubscription controller.
func NewConstructor(ipm iam.IAMPolicyManager, gcpas *gcpauth.StoreSingleton) Constructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newController(ctx, cmw, ipm, gcpas.Store(ctx, cmw))
	}
}

func newController(
	ctx context.Context,
	cmw configmap.Watcher,
	ipm iam.IAMPolicyManager,
	gcpas *gcpauth.Store,
) *controller.Impl {
	deploymentInformer := deploymentinformer.Get(ctx)
	pullSubscriptionInformer := pullsubscriptioninformers.Get(ctx)
	serviceAccountInformer := serviceaccountinformers.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName).Desugar()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	pullSubscriptionLister := pullSubscriptionInformer.Lister()

	r := &Reconciler{
		Base: &psreconciler.Base{
			Base:                   reconciler.NewBase(ctx, controllerAgentName, cmw),
			Identity:               identity.NewIdentity(ctx, ipm, gcpas),
			DeploymentLister:       deploymentInformer.Lister(),
			ServiceAccountLister:   serviceAccountInformer.Lister(),
			PullSubscriptionLister: pullSubscriptionLister,
			ReceiveAdapterImage:    env.ReceiveAdapter,
			CreateClientFn:         pubsub.NewClient,
			ControllerAgentName:    controllerAgentName,
			ResourceGroup:          resourceGroup,
		},
	}

	impl := pullsubscriptionreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")

	// Whenever we introduce a new way of scaling, this code will have to be updated to not just exclude Keda, but the others.
	// Might be useful to use pkgreconciler.ChainFilterFuncs and move them somewhere else.
	// TODO revisit once we introduce new scaling strategies.
	onlyKedaScaler := pkgreconciler.AnnotationFilterFunc(duck.AutoscalingClassAnnotation, duck.KEDA, false)
	notKedaScaler := pkgreconciler.Not(onlyKedaScaler)

	pullSubscriptionHandler := cache.FilteringResourceEventHandler{
		FilterFunc: notKedaScaler,
		Handler:    controller.HandleAll(impl.Enqueue),
	}
	pullSubscriptionInformer.Informer().AddEventHandlerWithResyncPeriod(pullSubscriptionHandler, reconciler.DefaultResyncPeriod)

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: notKedaScaler,
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// Watch k8s service account, if a k8s service account resource changes, enqueue qualified pullsubscriptions from the same namespace.
	serviceAccountInformer.Informer().AddEventHandler(authcheck.EnqueuePullSubscription(impl, pullSubscriptionLister))

	r.UriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)
	r.ReconcileDataPlaneFn = r.ReconcileDeployment

	cmw.Watch(logging.ConfigMapName(), r.UpdateFromLoggingConfigMap)
	cmw.Watch(metrics.ConfigMapName(), r.UpdateFromMetricsConfigMap)
	cmw.Watch(tracingconfig.ConfigName, r.UpdateFromTracingConfigMap)

	return impl
}
