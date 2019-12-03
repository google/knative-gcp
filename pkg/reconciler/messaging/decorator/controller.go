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

package decorator

import (
	"context"

	"knative.dev/pkg/resolver"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	serviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"

	decoratorinformer "github.com/google/knative-gcp/pkg/client/injection/informers/messaging/v1alpha1/decorator"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "Decorators"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-decorator-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	decoratorInformer := decoratorinformer.Get(ctx)
	serviceinformer := serviceinformer.Get(ctx)

	r := &Reconciler{
		Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
		decoratorLister: decoratorInformer.Lister(),
		serviceLister:   serviceinformer.Lister(),
	}

	impl := controller.NewImpl(r, r.Logger, reconcilerName)

	r.Logger.Info("Setting up event handlers")
	decoratorInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceinformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Decorator")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	return impl
}
