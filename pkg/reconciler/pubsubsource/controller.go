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

package pubsubsource

import (
	eventsinformers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/informers/externalversions/events/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/knative/pkg/controller"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-pubsub-source-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(
	opt reconciler.Options,
	sourceInformer eventsinformers.PubSubSourceInformer,
) *controller.Impl {

	c := &Reconciler{
		Base:         reconciler.NewBase(opt, controllerAgentName),
		sourceLister: sourceInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, ReconcilerName, nil)

	c.Logger.Info("Setting up event handlers")
	sourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	//configurationInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
	//	FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Service")),
	//	Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	//})
	//
	//routeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
	//	FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Service")),
	//	Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	//})

	return impl
}
