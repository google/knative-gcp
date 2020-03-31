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

package httppolicybinding

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	policyapi "github.com/google/knative-gcp/pkg/apis/policy"
	policyv1alpha1 "github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	policyinformer "github.com/google/knative-gcp/pkg/client/injection/informers/policy/v1alpha1/httppolicy"
	bindinginformer "github.com/google/knative-gcp/pkg/client/injection/informers/policy/v1alpha1/httppolicybinding"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/policy/v1alpha1/httppolicybinding"
	istioclient "github.com/google/knative-gcp/pkg/client/istio/injection/client"
	authzinformer "github.com/google/knative-gcp/pkg/client/istio/injection/informers/security/v1beta1/authorizationpolicy"
	authninformer "github.com/google/knative-gcp/pkg/client/istio/injection/informers/security/v1beta1/requestauthentication"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/policy"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName      = "IstioHTTPPolicyBindings"
	controllerAgentName = "istio-httppolicybinding-controller"
)

var httpPolicyGVK = policyv1alpha1.SchemeGroupVersion.WithKind("HTTPPolicy")

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	bindingInformer := bindinginformer.Get(ctx)
	policyInformer := policyinformer.Get(ctx)
	authzInformer := authzinformer.Get(ctx)
	authnInformer := authninformer.Get(ctx)

	r := &Reconciler{
		Base:         reconciler.NewBase(ctx, controllerAgentName, cmw),
		policyLister: policyInformer.Lister(),
		authzLister:  authzInformer.Lister(),
		authnLister:  authnInformer.Lister(),
		istioClient:  istioclient.Get(ctx),
	}

	impl := bindingreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")

	bindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.AnnotationFilterFunc(policyapi.PolicyBindingClassAnnotationKey, policyapi.IstioPolicyBindingClassValue, false),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	authzInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(policyv1alpha1.Kind("HTTPPolicyBinding")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	authnInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(policyv1alpha1.Kind("HTTPPolicyBinding")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	r.subjectResolver = policy.NewSubjectResolver(ctx, impl.EnqueueKey)

	r.policyTracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	policyInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(r.policyTracker.OnChanged, httpPolicyGVK),
	))

	return impl
}
