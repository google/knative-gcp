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

	channellister "github.com/google/knative-gcp/pkg/client/listers/messaging/v1beta1"

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"

	"github.com/google/knative-gcp/pkg/logging"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/celltenant"
	"github.com/google/knative-gcp/pkg/utils"

	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	brokercellinformer "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"

	"knative.dev/pkg/injection"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	channelinformer "github.com/google/knative-gcp/pkg/client/injection/informers/messaging/v1beta1/channel"
	channelreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/messaging/v1beta1/channel"
	"github.com/google/knative-gcp/pkg/reconciler"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-channel-controller"
)

type Constructor injection.ControllerConstructor

// NewConstructor creates a constructor to make a Channel controller.
func NewConstructor(drs *dataresidency.StoreSingleton) Constructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newController(ctx, cmw, drs.Store(ctx, cmw))
	}
}

func newController(
	ctx context.Context,
	cmw configmap.Watcher,
	drs *dataresidency.Store,
) *controller.Impl {
	channelInformer := channelinformer.Get(ctx)
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
		Reconciler: celltenant.Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			BrokerCellLister:   bcInformer.Lister(),
			ProjectID:          projectID,
			PubsubClient:       client,
			DataresidencyStore: drs,
		},
		targetReconciler: &celltenant.TargetReconciler{
			ProjectID:          projectID,
			PubsubClient:       client,
			DataresidencyStore: drs,
		},
	}
	impl := channelreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandlerWithResyncPeriod(
		controller.HandleAll(impl.Enqueue), reconciler.DefaultResyncPeriod)

	bcInformer.Informer().AddEventHandler(controller.HandleAll(filterChannelsForBrokerCell(
		r.Logger.Desugar(), channelInformer.Lister(), impl.Enqueue)))

	return impl
}

// filterChannelsForBrokerCell creates a filter that is intended to be used on the BrokerCell
//  informer. It will enqueue all Channels associated with the changed BrokerCell.
func filterChannelsForBrokerCell(
	logger *zap.Logger, channelInformer channellister.ChannelLister, enqueue func(interface{})) func(obj interface{}) {
	return func(obj interface{}) {
		if _, ok := obj.(*inteventsv1alpha1.BrokerCell); ok {
			// TODO(#866) Only select brokers that point to this brokercell by label selector once the
			// webhook assigns the brokercell label, i.e.,
			// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
			channels, err := channelInformer.List(labels.Everything())
			if err != nil {
				logger.Error("Failed to list Channels", zap.Error(err))
				return
			}
			for _, channel := range channels {
				enqueue(channel)
			}
		}
	}
}
