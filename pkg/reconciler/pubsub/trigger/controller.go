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

	"k8s.io/client-go/tools/cache"
	serviceaccountinformers "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
	triggerinformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1beta1/trigger"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/pubsub/v1beta1/trigger"
	gtrigger "github.com/google/knative-gcp/pkg/gclient/trigger"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "Trigger"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-trigger-controller"

	// receiveAdapterName is the string used as name for the receive adapter pod.
	receiveAdapterName = "trigger.pubsub.cloud.google.com"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	triggerInformer := triggerinformers.Get(ctx)
	serviceAccountInformer := serviceaccountinformers.Get(ctx)

	r := &Reconciler{
		PubSubBase:           pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
		Identity:             identity.NewIdentity(ctx, iam.DefaultIAMPolicyManager()),
		triggerLister:        triggerInformer.Lister(),
		serviceAccountLister: serviceAccountInformer.Lister(),
		createClientFn:       gtrigger.NewClient,
	}
	impl := triggerreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")
	triggerInformer.Informer().AddEventHandlerWithResyncPeriod(controller.HandleAll(impl.Enqueue), reconciler.DefaultResyncPeriod)

	serviceAccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupVersionKind(v1beta1.SchemeGroupVersion.WithKind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
