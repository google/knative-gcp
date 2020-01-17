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

package storage

import (
	"context"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	gstorage "github.com/google/knative-gcp/pkg/gclient/storage"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	storageinformers "github.com/google/knative-gcp/pkg/client/injection/informers/events/v1alpha1/storage"
	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
	topicinformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/topic"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "CloudStorageSource"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-storage-source-controller"

	// receiveAdapterName is the string used as name for the receive adapter pod.
	receiveAdapterName = "cloudstoragesource.events.cloud.google.com"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	pullsubscriptionInformer := pullsubscriptioninformers.Get(ctx)
	topicInformer := topicinformers.Get(ctx)
	storageInformer := storageinformers.Get(ctx)

	r := &Reconciler{
		PubSubBase:     pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
		storageLister:  storageInformer.Lister(),
		createClientFn: gstorage.NewClient,
	}
	impl := controller.NewImpl(r, r.Logger, reconcilerName)

	r.Logger.Info("Setting up event handlers")
	storageInformer.Informer().AddEventHandlerWithResyncPeriod(controller.HandleAll(impl.Enqueue), reconciler.DefaultResyncPeriod)

	topicInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CloudStorageSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	pullsubscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CloudStorageSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
