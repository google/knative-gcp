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

package build

import (
	"context"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	cloudbuildsourceinformers "github.com/google/knative-gcp/pkg/client/injection/informers/events/v1alpha1/cloudbuildsource"
	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
	cloudbuildsourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudbuildsource"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"k8s.io/client-go/tools/cache"
	serviceaccountinformers "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "CloudBuildSource"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-build-source-controller"

	// receiveAdapterName is the string used as name for the receive adapter pod.
	receiveAdapterName = "cloudbuildsource.events.cloud.google.com"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	pullsubscriptionInformer := pullsubscriptioninformers.Get(ctx)
	cloudbuildsourceInformer := cloudbuildsourceinformers.Get(ctx)
	serviceAccountInformer := serviceaccountinformers.Get(ctx)

	r := &Reconciler{
		PubSubBase:             pubsub.NewPubSubBaseWithAdapter(ctx, controllerAgentName, receiveAdapterName, converters.CloudBuildConverter, cmw),
		Identity:               identity.NewIdentity(ctx),
		buildLister:            cloudbuildsourceInformer.Lister(),
		serviceAccountLister:   serviceAccountInformer.Lister(),
		pullsubscriptionLister: pullsubscriptionInformer.Lister(),
	}
	impl := cloudbuildsourcereconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")
	cloudbuildsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	pullsubscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind("CloudBuildSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
