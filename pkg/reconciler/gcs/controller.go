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

package gcs

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	//	"knative.dev/pkg/logging"
	//	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"

	jobinformer "knative.dev/pkg/injection/informers/kubeinformers/batchv1/job"

	storageinformers "github.com/google/knative-gcp/pkg/client/injection/informers/events/v1alpha1/storage"
	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-storage-source-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	pullsubscriptionInformer := pullsubscriptioninformers.Get(ctx)
	jobInformer := jobinformer.Get(ctx)
	storageInformer := storageinformers.Get(ctx)

	c := &Reconciler{
		Base:                   reconciler.NewBase(ctx, controllerAgentName, cmw),
		storageLister:          storageInformer.Lister(),
		pullSubscriptionLister: pullsubscriptionInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers")
	storageInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	jobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Storage")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	//	c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	return impl
}
