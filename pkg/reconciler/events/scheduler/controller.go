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

package scheduler

import (
	"context"

	"knative.dev/pkg/injection"

	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	"k8s.io/client-go/tools/cache"
	serviceaccountinformers "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	cloudschedulersourceinformers "github.com/google/knative-gcp/pkg/client/injection/informers/events/v1/cloudschedulersource"
	pullsubscriptioninformers "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1/pullsubscription"
	topicinformers "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1/topic"
	cloudschedulersourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudschedulersource"
	gscheduler "github.com/google/knative-gcp/pkg/gclient/scheduler"
)

const (
	// reconcilerName is the name of the reconciler
	reconcilerName = "CloudSchedulerSource"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-scheduler-source-controller"

	// receiveAdapterName is the string used as name for the receive adapter pod.
	receiveAdapterName = "cloudschedulersource.events.cloud.google.com"
)

type Constructor injection.ControllerConstructor

// NewConstructor creates a constructor to make a CloudSchedulerSource controller.
func NewConstructor(ipm iam.IAMPolicyManager, gcpas *gcpauth.StoreSingleton) Constructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newController(ctx, cmw, ipm, gcpas.Store(ctx, cmw))
	}
}

func newController(
	ctx context.Context,
	cmw configmap.Watcher,
	ipm iam.IAMPolicyManager,
	gcpas *gcpauth.Store,
) *controller.Impl {
	pullsubscriptionInformer := pullsubscriptioninformers.Get(ctx)
	topicInformer := topicinformers.Get(ctx)
	cloudschedulersourceInformer := cloudschedulersourceinformers.Get(ctx)
	serviceAccountInformer := serviceaccountinformers.Get(ctx)

	c := &Reconciler{
		PubSubBase: intevents.NewPubSubBase(ctx,
			&intevents.PubSubBaseArgs{
				ControllerAgentName: controllerAgentName,
				ReceiveAdapterName:  receiveAdapterName,
				ReceiveAdapterType:  string(converters.CloudScheduler),
				ConfigWatcher:       cmw,
			}),
		Identity:        identity.NewIdentity(ctx, ipm, gcpas),
		schedulerLister: cloudschedulersourceInformer.Lister(),
		createClientFn:  gscheduler.NewClient,
	}
	impl := cloudschedulersourcereconciler.NewImpl(ctx, c)

	c.Logger.Info("Setting up event handlers")
	cloudschedulersourceInformer.Informer().AddEventHandlerWithResyncPeriod(controller.HandleAll(impl.Enqueue), reconciler.DefaultResyncPeriod)

	schedulerGK := v1.Kind("CloudSchedulerSource")

	topicInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(schedulerGK),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	pullsubscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(schedulerGK),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	serviceAccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(schedulerGK),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
