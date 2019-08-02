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

package topic

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"

	jobinformer "knative.dev/pkg/injection/informers/kubeinformers/batchv1/job"
	v1alpha1serviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1alpha1/service"

	topicinformer "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/injection/informers/pubsub/v1alpha1/topic"
	//v1beta1serviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1beta1/service"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloud-run-events-pubsub-topic-controller"
)

type envConfig struct {
	// Publisher is the image used to publish to Pub/Sub. Required.
	Publisher string `envconfig:"PUBSUB_PUBLISHER_IMAGE" required:"true"`

	// TopicOps is the image for operating on topics. Required.
	TopicOps string `envconfig:"PUBSUB_TOPIC_IMAGE" required:"true"`

	// TargetServingVersion is the version of Service.sering.knative.dev to use. Required.
	TargetServingVersion string `envconfig:"KN_SERVING_VERSION" default:"v1alpha1" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	topicInformer := topicinformer.Get(ctx)
	jobInformer := jobinformer.Get(ctx)
	v1alpha1ServiceInformer := v1alpha1serviceinformer.Get(ctx)
	//v1beta1ServiceInformer := v1beta1serviceinformer.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	pubsubBase := &pubsub.PubSubBase{
		Base:          reconciler.NewBase(ctx, controllerAgentName, cmw),
		TopicOpsImage: env.TopicOps,
	}

	c := &Reconciler{
		PubSubBase:     pubsubBase,
		topicLister:    topicInformer.Lister(),
		servingVersion: env.TargetServingVersion,
		publisherImage: env.Publisher,
	}

	switch c.servingVersion {
	case "", "v1alpha1":
		c.serviceV1alpha1Lister = v1alpha1ServiceInformer.Lister()
	case "v1beta1":
		logger.Error("v1beta1 serving version not supported until k8s 1.14")
		//c.serviceV1beta1Lister = v1beta1ServiceInformer.Lister()
	default:
		logger.Error("unknown serving version selected: %q", c.servingVersion)
	}

	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers")
	topicInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	switch c.servingVersion {
	case "", "v1alpha1":
		v1alpha1ServiceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Topic")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})
	case "v1beta1":
		logger.Error("v1beta1 serving version not supported until k8s 1.14")
		//v1beta1ServiceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		//	FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Topic")),
		//	Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		//})
	default:
		logger.Error("unknown serving version selected: %q", c.servingVersion)
	}

	jobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Topic")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
