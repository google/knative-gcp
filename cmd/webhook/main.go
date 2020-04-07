/*
Copyright 2019 The Knative Authors

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

package main

import (
	"context"

	configvalidation "github.com/google/knative-gcp/pkg/apis/configs/validation"
	eventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/pubsub"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	pubsubv1beta1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	tracingconfig "knative.dev/pkg/tracing/config"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/conversion"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// For group messaging.cloud.google.com.
	messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"): &messagingv1alpha1.Channel{},

	// For group events.cloud.google.com.
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudStorageSource"):   &eventsv1alpha1.CloudStorageSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudSchedulerSource"): &eventsv1alpha1.CloudSchedulerSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudPubSubSource"):    &eventsv1alpha1.CloudPubSubSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudAuditLogsSource"): &eventsv1alpha1.CloudAuditLogsSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudBuildSource"):     &eventsv1alpha1.CloudBuildSource{},

	// For group pubsub.cloud.google.com.
	pubsubv1alpha1.SchemeGroupVersion.WithKind("PullSubscription"): &pubsubv1alpha1.PullSubscription{},
	pubsubv1alpha1.SchemeGroupVersion.WithKind("Topic"):            &pubsubv1alpha1.Topic{},
}

func NewDefaultingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return defaulting.NewAdmissionController(ctx,

		// Name of the default webhook.
		"webhook.events.cloud.google.com",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to validate and default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,

		// Name of the validation webhook.
		"validation.webhook.events.cloud.google.com",

		// The path on which to serve the webhook.
		"/validation",

		// The resources to validate and default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			// return v1.WithUpgradeViaDefaulting(store.ToContext(ctx))
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func NewConfigValidationController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,

		// Name of the configmap webhook.
		"config.webhook.events.cloud.google.com",

		// The path on which to serve the webhook.
		"/config-validation",

		// The configmaps to validate.
		configmap.Constructors{
			tracingconfig.ConfigName: tracingconfig.NewTracingConfigFromConfigMap,
			// metrics.ConfigMapName():   metricsconfig.NewObservabilityConfigFromConfigMap,
			logging.ConfigMapName():        logging.NewConfigFromConfigMap,
			leaderelection.ConfigMapName(): configvalidation.ValidateLeaderElectionConfig,
		},
	)
}

func NewConversionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	var (
		pubsubv1alpha1_ = pubsubv1alpha1.SchemeGroupVersion.Version
		pubsubv1beta1_  = pubsubv1beta1.SchemeGroupVersion.Version
	)

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			// pubsub
			pubsubv1beta1.Kind("PullSubscription"): {
				DefinitionName: pubsub.PullSubscriptionsResource.String(),
				HubVersion:     pubsubv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					pubsubv1alpha1_: &pubsubv1alpha1.PullSubscription{},
					pubsubv1beta1_:  &pubsubv1beta1.PullSubscription{},
				},
			},
			pubsubv1beta1.Kind("Topic"): {
				DefinitionName: pubsub.TopicsResource.String(),
				HubVersion:     pubsubv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					pubsubv1alpha1_: &pubsubv1alpha1.Topic{},
					pubsubv1beta1_:  &pubsubv1beta1.Topic{},
				},
			},
		},
		func(ctx context.Context) context.Context {
			return ctx
		},
	)
}
func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: logconfig.WebhookName(),
		Port:        8443,
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, logconfig.WebhookName(),
		certificates.NewController,
		NewConfigValidationController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
		NewConversionController,
	)
}
