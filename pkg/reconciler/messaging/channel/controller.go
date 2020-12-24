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

	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	brokercellinformer "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"

	"knative.dev/pkg/injection"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	channelinformer "github.com/google/knative-gcp/pkg/client/injection/informers/messaging/v1beta1/channel"
	channelreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/messaging/v1beta1/channel"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "Channels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-channel-controller"
)

type Constructor injection.ControllerConstructor

// NewConstructor creates a constructor to make a Channel controller.
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
	channelInformer := channelinformer.Get(ctx)
	bcInformer := brokercellinformer.Get(ctx)

	r := &Reconciler{
		Base:             reconciler.NewBase(ctx, controllerAgentName, cmw),
		Identity:         identity.NewIdentity(ctx, ipm, gcpas),
		channelLister:    channelInformer.Lister(),
		brokerCellLister: bcInformer.Lister(),
	}
	impl := channelreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	bcInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if _, ok := obj.(*inteventsv1alpha1.BrokerCell); ok {
				// TODO(#866) Only select brokers that point to this brokercell by label selector once the
				// webhook assigns the brokercell label, i.e.,
				// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
				channels, err := channelInformer.Lister().List(labels.Everything())
				if err != nil {
					r.Logger.Desugar().Error("Failed to list Channels", zap.Error(err))
					return
				}
				for _, channel := range channels {
					impl.Enqueue(channel)
				}
			}
		},
	))
	return impl
}
