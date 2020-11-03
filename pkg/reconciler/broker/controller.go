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

	"github.com/google/knative-gcp/pkg/apis/configs/brokerdelivery"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"

	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/google/knative-gcp/pkg/logging"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	pkgreconciler "knative.dev/pkg/reconciler"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	brokerinformer "github.com/google/knative-gcp/pkg/client/injection/informers/broker/v1beta1/broker"
	brokercellinformer "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell"
	brokerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/broker"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "broker-controller"
)

type Constructor injection.ControllerConstructor

// NewConstructor creates a constructor to make a Broker controller.
func NewConstructor(brokerdeliveryss *brokerdelivery.StoreSingleton, dataresidencyss *dataresidency.StoreSingleton) Constructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newController(ctx, cmw, brokerdeliveryss.Store(ctx, cmw), dataresidencyss.Store(ctx, cmw))
	}
}

func newController(ctx context.Context, cmw configmap.Watcher, brds *brokerdelivery.Store, drs *dataresidency.Store) *controller.Impl {
	brokerInformer := brokerinformer.Get(ctx)
	bcInformer := brokercellinformer.Get(ctx)

	var client *pubsub.Client
	// If there is an error, the projectID will be empty. The reconciler will retry
	// to get the projectID during reconciliation.
	projectID, err := utils.ProjectIDOrDefault("")
	if err != nil {
		logging.FromContext(ctx).Error("Failed to get project ID", zap.Error(err))
	} else {
		// Attempt to create a pubsub client for all worker threads to use. If this
		// fails, pass a nil value to the Reconciler. They will attempt to
		// create a client on reconcile.
		if client, err = pubsub.NewClient(ctx, projectID); err != nil {
			client = nil
			logging.FromContext(ctx).Error("Failed to create controller-wide Pub/Sub client", zap.Error(err))
		}
	}

	if client != nil {
		go func() {
			<-ctx.Done()
			client.Close()
		}()
	}

	r := &Reconciler{
		Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
		brokerCellLister:   bcInformer.Lister(),
		pubsubClient:       client,
		dataresidencyStore: drs,
	}

	impl := brokerreconciler.NewImpl(ctx, r, brokerv1beta1.BrokerClass,
		func(impl *controller.Impl) controller.Options {
			return controller.Options{
				ConfigStore: brds,
			}
		})

	r.Logger.Info("Setting up event handlers")

	brokerInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			// Only reconcile brokers with the proper class annotation
			FilterFunc: pkgreconciler.AnnotationFilterFunc(eventingv1beta1.BrokerClassAnnotationKey, brokerv1beta1.BrokerClass, false /*allowUnset*/),
			Handler:    controller.HandleAll(impl.Enqueue),
		},
		reconciler.DefaultResyncPeriod,
	)

	bcInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if _, ok := obj.(*inteventsv1alpha1.BrokerCell); ok {
				// TODO(#866) Only select brokers that point to this brokercell by label selector once the
				// webhook assigns the brokercell label, i.e.,
				// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
				brokers, err := brokerInformer.Lister().List(labels.Everything())
				if err != nil {
					r.Logger.Error("Failed to list brokers", zap.Error(err))
					return
				}
				for _, broker := range brokers {
					impl.Enqueue(broker)
				}
			}
		},
	))

	return impl
}
