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
package deployment

import (
	"context"

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/secret"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Deployment"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-deployment-controller"

	namespace      = "cloud-run-events"
	secretName     = v1alpha1.DefaultSecretName
	deploymentName = "controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events.
// When the secret `google-cloud-key` of namespace `cloud-run-events` gets updated, we will enqueue the deployment `controller` of namespace `cloud-run-events`.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	deploymentInformer := deployment.Get(ctx)
	secretInformer := secret.Get(ctx)

	r := &Reconciler{
		Base:             reconciler.NewBase(ctx, controllerAgentName, cmw),
		deploymentLister: deploymentInformer.Lister(),
		clock:            clock.RealClock{},
	}

	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	sentinel := impl.EnqueueSentinel(types.NamespacedName{Name: deploymentName, Namespace: namespace})
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(namespace, secretName),
		Handler:    handleUpdateOnly(sentinel),
	})
	return impl
}

func handleUpdateOnly(h func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    doNothing,
		UpdateFunc: controller.PassNew(h),
		DeleteFunc: doNothing,
	}
}

func doNothing(obj interface{}) {
}
