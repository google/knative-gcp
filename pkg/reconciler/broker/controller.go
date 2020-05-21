/*
Copyright 2020 Google LLC

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

package broker

import (
	"context"

	"cloud.google.com/go/pubsub"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	injectionclient "github.com/google/knative-gcp/pkg/client/injection/client"
	brokerinformer "github.com/google/knative-gcp/pkg/client/injection/informers/broker/v1beta1/broker"
	triggerinformer "github.com/google/knative-gcp/pkg/client/injection/informers/broker/v1beta1/trigger"
	brokercellinformer "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell"
	brokerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/broker"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/trigger"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/utils"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	endpointsinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "broker-controller"
)

func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	brokerInformer := brokerinformer.Get(ctx)
	triggerInformer := triggerinformer.Get(ctx)
	configMapInformer := configmapinformer.Get(ctx)
	endpointsInformer := endpointsinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	podInformer := podinformer.Get(ctx)
	bcInformer := brokercellinformer.Get(ctx)

	// Attempt to create a pubsub client for all worker threads to use. If this
	// fails, pass a nil value to the Reconciler. They will attempt to
	// create a client on reconcile.
	client, err := newPubsubClient(ctx, "")
	if err != nil {
		logging.FromContext(ctx).Error("Failed to create controller-wide Pub/Sub client", zap.Error(err))
	}

	if client != nil {
		go func() {
			<-ctx.Done()
			client.Close()
		}()
	}

	r := &Reconciler{
		Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
		triggerLister:      triggerInformer.Lister(),
		configMapLister:    configMapInformer.Lister(),
		endpointsLister:    endpointsInformer.Lister(),
		deploymentLister:   deploymentInformer.Lister(),
		podLister:          podInformer.Lister(),
		brokerCellLister:   bcInformer.Lister(),
		pubsubClient:       client,
		targetsNeedsUpdate: make(chan struct{}),
	}

	//TODO wrap this up in a targets struct backed by a configmap
	// Load targets config from the existing configmap if present
	if err := r.LoadTargetsConfig(ctx); err != nil {
		r.Logger.Error("error loading targets config", zap.Error(err))
		// For some reason the targets config is corrupt, proceed with an
		// empty one
		r.targetsConfig = memory.NewEmptyTargets()
	}

	// Start the single thread updating the targets configmap
	go r.TargetsConfigUpdater(ctx)

	impl := brokerreconciler.NewImpl(ctx, r, brokerv1beta1.BrokerClass)

	tr := &TriggerReconciler{
		Base:         reconciler.NewBase(ctx, controllerAgentName, cmw),
		pubsubClient: client,
	}

	triggerReconciler := triggerreconciler.NewReconciler(
		ctx,
		r.Logger,
		injectionclient.Get(ctx),
		triggerInformer.Lister(),
		r.Recorder,
		tr,
	)

	r.triggerReconciler = triggerReconciler

	r.Logger.Info("Setting up event handlers")

	tr.kresourceTracker = duck.NewListableTracker(ctx, conditions.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	tr.addressableTracker = duck.NewListableTracker(ctx, addressable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	tr.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	brokerInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			// Only reconcile brokers with the proper class annotation
			FilterFunc: pkgreconciler.AnnotationFilterFunc(eventingv1beta1.BrokerClassAnnotationKey, brokerv1beta1.BrokerClass, false /*allowUnset*/),
			Handler:    controller.HandleAll(impl.Enqueue),
		},
		reconciler.DefaultResyncPeriod,
	)

	// Don't watch the targets configmap because it would require reconciling
	// all brokers every update. In normal operation this
	// will never be modified except by the controller. The global resync
	// will resync all brokers every 5 minutes, correcting any issues caused
	// by users.
	//configMapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
	//	FilterFunc: pkgreconciler.LabelExistsFilterFunc(eventing.BrokerLabelKey),
	//	Handler:    controller.HandleAll(impl.EnqueueLabelOfNamespaceScopedResource("" /*any namespace*/, eventing.BrokerLabelKey)),
	//})

	//TODO https://github.com/knative/eventing/pull/2779/files
	//TODO Need to watch only the shared ingress
	// endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
	// 	FilterFunc: pkgreconciler.LabelExistsFilterFunc(eventing.BrokerLabelKey),
	// 	Handler:    controller.HandleAll(impl.EnqueueLabelOfNamespaceScopedResource("" /*any namespace*/, eventing.BrokerLabelKey)),
	// })

	//TODO Also reconcile triggers when their broker doesn't exist. Maybe use a
	// synthetic broker and call reconcileTriggers anyway?
	// How do we do this? We can check the lister to see if the broker exists
	// and do something different if it does not, but that still allows the
	// broker to be deleted while the reconcile is in the queue. Need a workaround
	// for the gen reconciler not reconciling when the reconciled object doesn't exist.

	// Maybe we need to override the genreconciler's Reconcile method to go ahead and reconcile
	// if the broker doesn't exist.

	// Is there a race if we create a separate controller for the trigger reconciler?
	// Yes, because the broker and trigger controller could be reconciling at the same time
	// If we want the broker controller to continue being responsible for reconciling all triggers,
	// we need the ability to reconcile objects that don't exist
	triggerInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if trigger, ok := obj.(*brokerv1beta1.Trigger); ok {
				impl.EnqueueKey(types.NamespacedName{Namespace: trigger.Namespace, Name: trigger.Spec.Broker})
			}
		},
	))

	return impl
}

func newPubsubClient(ctx context.Context, projectID string) (*pubsub.Client, error) {
	projectID, err := utils.ProjectID(projectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		return nil, err
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return client, nil
}
