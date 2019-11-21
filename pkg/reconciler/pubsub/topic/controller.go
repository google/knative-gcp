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
	tracingconfig "knative.dev/pkg/tracing/config"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/events/pubsub"

	topicinformer "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/topic"
	serviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"
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
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	topicInformer := topicinformer.Get(ctx)
	serviceinformer := serviceinformer.Get(ctx)

	logger := logging.FromContext(ctx).Named(controllerAgentName).Desugar()

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
		serviceLister:  serviceinformer.Lister(),
		publisherImage: env.Publisher,
	}

	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	logger.Info("Setting up event handlers")
	topicInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceinformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Topic")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cmw.Watch(tracingconfig.ConfigName, c.UpdateFromTracingConfigMap)

	return impl
}
