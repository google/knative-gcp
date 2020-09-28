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

package trigger

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/google/knative-gcp/pkg/logging"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	pkgcontroller "knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	brokerinformer "github.com/google/knative-gcp/pkg/client/injection/informers/broker/v1beta1/broker"
	triggerinformer "github.com/google/knative-gcp/pkg/client/injection/informers/broker/v1beta1/trigger"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/trigger"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "trigger-controller"
	// finalizerName is the name of the finalizer that this controller adds to the Triggers that it reconciles.
	finalizerName = "googlecloud"
)

// filterBroker is the function to filter brokers with proper brokerclass.
var filterBroker = pkgreconciler.AnnotationFilterFunc(eventingv1beta1.BrokerClassAnnotationKey, brokerv1beta1.BrokerClass, false /*allowUnset*/)

type envConfig struct {
	ProjectID string `envconfig:"PROJECT_ID"`
}

type Constructor injection.ControllerConstructor

// NewConstructor creates a constructor to make a Trigger controller.
func NewConstructor(dataresidencyss *dataresidency.StoreSingleton) Constructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newController(ctx, cmw, dataresidencyss.Store(ctx, cmw))
	}
}

func newController(ctx context.Context, cmw configmap.Watcher, drs *dataresidency.Store) *controller.Impl {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", zap.Error(err))
	}

	triggerInformer := triggerinformer.Get(ctx)

	// If there is an error, the projectID will be empty. The reconciler will retry
	// to get the projectID during reconciliation.
	projectID, err := utils.ProjectID(env.ProjectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		logging.FromContext(ctx).Error("Failed to get project ID", zap.Error(err))
	}

	// Attempt to create a pubsub client for all worker threads to use. If this
	// fails, pass a nil value to the Reconciler. They will attempt to
	// create a client on reconcile.
	client, err := newPubsubClient(ctx, projectID)
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
		brokerLister:       brokerinformer.Get(ctx).Lister(),
		pubsubClient:       client,
		projectID:          projectID,
		dataresidencyStore: drs,
	}

	impl := triggerreconciler.NewImpl(ctx, r, withAgentAndFinalizer)
	r.kresourceTracker = duck.NewListableTracker(ctx, conditions.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.addressableTracker = duck.NewListableTracker(ctx, addressable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	r.Logger.Info("Setting up event handlers")

	triggerInformer.Informer().AddEventHandlerWithResyncPeriod(controller.HandleAll(impl.Enqueue), reconciler.DefaultResyncPeriod)

	// Watch brokers.
	brokerinformer.Get(ctx).Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			// Only care about brokers with the proper class annotation
			FilterFunc: filterBroker,
			Handler: controller.HandleAll(func(obj interface{}) {
				if b, ok := obj.(*brokerv1beta1.Broker); ok {
					triggers, err := triggerinformer.Get(ctx).Lister().Triggers(b.Namespace).List(labels.SelectorFromSet(map[string]string{eventing.BrokerLabelKey: b.Name}))
					if err != nil {
						r.Logger.Warn("Failed to list triggers", zap.String("Namespace", b.Namespace), zap.String("Broker", b.Name))
						return
					}
					for _, trigger := range triggers {
						impl.Enqueue(trigger)
					}
				}
			}),
		},
	)

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

func withAgentAndFinalizer(impl *pkgcontroller.Impl) pkgcontroller.Options {
	return pkgcontroller.Options{
		FinalizerName: finalizerName,
		AgentName:     controllerAgentName,
	}
}
