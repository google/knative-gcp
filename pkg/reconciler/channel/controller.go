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

package channel

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"

	channelinformer "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/injection/informers/events/v1alpha1/channel"
	subscriptioninformer "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription"
	topicinformer "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/injection/informers/pubsub/v1alpha1/topic"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-channel-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	channelInformer := channelinformer.Get(ctx)

	topicInformer := topicinformer.Get(ctx)
	subscriptionInformer := subscriptioninformer.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName)
	_ = logger

	c := &Reconciler{
		Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
		topicLister:        topicInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	topicInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Channel")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Channel")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	c.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	return impl
}
