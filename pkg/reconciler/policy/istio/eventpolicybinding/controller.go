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

package eventpolicybinding

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	policyapi "github.com/google/knative-gcp/pkg/apis/policy"
	policyv1alpha1 "github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	eventpolicyinformer "github.com/google/knative-gcp/pkg/client/injection/informers/policy/v1alpha1/eventpolicy"
	eventpolicybindinginformer "github.com/google/knative-gcp/pkg/client/injection/informers/policy/v1alpha1/eventpolicybinding"
	httppolicyinformer "github.com/google/knative-gcp/pkg/client/injection/informers/policy/v1alpha1/httppolicy"
	httppolicybindinginformer "github.com/google/knative-gcp/pkg/client/injection/informers/policy/v1alpha1/httppolicybinding"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/policy/v1alpha1/eventpolicybinding"
	"github.com/google/knative-gcp/pkg/reconciler"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName      = "IstioEventPolicyBindings"
	controllerAgentName = "istio-eventpolicybinding-controller"
)

var eventPolicyGVK = policyv1alpha1.SchemeGroupVersion.WithKind("EventPolicy")

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	eventPolicyBindingInformer := eventpolicybindinginformer.Get(ctx)
	httpPolicyBindingInformer := httppolicybindinginformer.Get(ctx)
	eventPolicyInformer := eventpolicyinformer.Get(ctx)
	httpPolicyInformer := httppolicyinformer.Get(ctx)

	r := &Reconciler{
		Base:                    reconciler.NewBase(ctx, controllerAgentName, cmw),
		eventPolicyLister:       eventPolicyInformer.Lister(),
		httpPolicyLister:        httpPolicyInformer.Lister(),
		httpPolicyBindingLister: httpPolicyBindingInformer.Lister(),
	}

	impl := bindingreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")

	eventPolicyBindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.AnnotationFilterFunc(policyapi.PolicyBindingClassAnnotationKey, policyapi.IstioPolicyBindingClassValue, false),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	httpPolicyBindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(policyv1alpha1.Kind("EventPolicyBinding")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	httpPolicyInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(policyv1alpha1.Kind("EventPolicyBinding")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	r.policyTracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	eventPolicyInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(r.policyTracker.OnChanged, eventPolicyGVK),
	))

	return impl
}
