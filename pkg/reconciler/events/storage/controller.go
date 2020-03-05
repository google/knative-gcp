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

	cloudstoragesourceinformers "github.com/google/knative-gcp/pkg/client/injection/informers/events/v1alpha1/cloudstoragesource"
	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
	topicinformers "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/topic"
	cloudstoragesourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudstoragesource"
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
	cloudstoragesourceInformer := cloudstoragesourceinformers.Get(ctx)

	r := &Reconciler{
		PubSubBase:     pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
		storageLister:  cloudstoragesourceInformer.Lister(),
		createClientFn: gstorage.NewClient,
	}
	impl := cloudstoragesourcereconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")
	cloudstoragesourceInformer.Informer().AddEventHandlerWithResyncPeriod(controller.HandleAll(impl.Enqueue), reconciler.DefaultResyncPeriod)

	// Call GlobalResync on pubsubsource.
	grCh := func(obj interface{}) {
		impl.GlobalResync(cloudstoragesourceInformer.Informer())
	}

	topicInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CloudStorageSource")),
		Handler:    controller.HandleAll(grCh),
	})

	pullsubscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CloudStorageSource")),
		Handler:    controller.HandleAll(grCh),
	})

	return impl
}
