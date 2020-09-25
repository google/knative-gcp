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
	"log"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/configs/broker"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	"github.com/google/knative-gcp/pkg/apis/events"
	eventsv1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	eventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	eventsv1beta1 "github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/intevents"
	inteventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	inteventsv1beta1 "github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/messaging"
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	messagingv1beta1 "github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
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
	// For group eventing.knative.dev.
	brokerv1beta1.SchemeGroupVersion.WithKind("Broker"): &brokerv1beta1.Broker{},

	// For group messaging.cloud.google.com.
	messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"): &messagingv1alpha1.Channel{},
	messagingv1beta1.SchemeGroupVersion.WithKind("Channel"):  &messagingv1beta1.Channel{},

	// For group events.cloud.google.com.
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudStorageSource"):   &eventsv1alpha1.CloudStorageSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudSchedulerSource"): &eventsv1alpha1.CloudSchedulerSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudPubSubSource"):    &eventsv1alpha1.CloudPubSubSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudAuditLogsSource"): &eventsv1alpha1.CloudAuditLogsSource{},
	eventsv1alpha1.SchemeGroupVersion.WithKind("CloudBuildSource"):     &eventsv1alpha1.CloudBuildSource{},
	eventsv1beta1.SchemeGroupVersion.WithKind("CloudStorageSource"):    &eventsv1beta1.CloudStorageSource{},
	eventsv1beta1.SchemeGroupVersion.WithKind("CloudSchedulerSource"):  &eventsv1beta1.CloudSchedulerSource{},
	eventsv1beta1.SchemeGroupVersion.WithKind("CloudPubSubSource"):     &eventsv1beta1.CloudPubSubSource{},
	eventsv1beta1.SchemeGroupVersion.WithKind("CloudAuditLogsSource"):  &eventsv1beta1.CloudAuditLogsSource{},
	eventsv1beta1.SchemeGroupVersion.WithKind("CloudBuildSource"):      &eventsv1beta1.CloudBuildSource{},
	eventsv1.SchemeGroupVersion.WithKind("CloudStorageSource"):         &eventsv1.CloudStorageSource{},
	eventsv1.SchemeGroupVersion.WithKind("CloudSchedulerSource"):       &eventsv1.CloudSchedulerSource{},
	eventsv1.SchemeGroupVersion.WithKind("CloudPubSubSource"):          &eventsv1.CloudPubSubSource{},
	eventsv1.SchemeGroupVersion.WithKind("CloudAuditLogsSource"):       &eventsv1.CloudAuditLogsSource{},
	eventsv1.SchemeGroupVersion.WithKind("CloudBuildSource"):           &eventsv1.CloudBuildSource{},

	// For group internal.events.cloud.google.com.
	inteventsv1alpha1.SchemeGroupVersion.WithKind("PullSubscription"): &inteventsv1alpha1.PullSubscription{},
	inteventsv1alpha1.SchemeGroupVersion.WithKind("Topic"):            &inteventsv1alpha1.Topic{},
	inteventsv1beta1.SchemeGroupVersion.WithKind("PullSubscription"):  &inteventsv1beta1.PullSubscription{},
	inteventsv1beta1.SchemeGroupVersion.WithKind("Topic"):             &inteventsv1beta1.Topic{},
	inteventsv1.SchemeGroupVersion.WithKind("PullSubscription"):       &inteventsv1.PullSubscription{},
	inteventsv1.SchemeGroupVersion.WithKind("Topic"):                  &inteventsv1.Topic{},
	inteventsv1alpha1.SchemeGroupVersion.WithKind("BrokerCell"):       &inteventsv1alpha1.BrokerCell{},
}

type defaultingAdmissionController func(context.Context, configmap.Watcher) *controller.Impl

func newDefaultingAdmissionConstructor(brokers *broker.StoreSingleton, gcpas *gcpauth.StoreSingleton) defaultingAdmissionController {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newDefaultingAdmissionController(ctx, cmw, brokers.Store(ctx, cmw), gcpas.Store(ctx, cmw))
	}
}

func newDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher, brokers *broker.Store, gcpas *gcpauth.Store) *controller.Impl {
	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return brokers.ToContext(gcpas.ToContext(ctx))
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

type validationController func(context.Context, configmap.Watcher) *controller.Impl

func newValidationConstructor(brokers *broker.StoreSingleton, gcpas *gcpauth.StoreSingleton) validationController {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newValidationAdmissionController(ctx, cmw, brokers.Store(ctx, cmw), gcpas.Store(ctx, cmw))
	}
}

func newValidationAdmissionController(ctx context.Context, cmw configmap.Watcher, brokers *broker.Store, gcpas *gcpauth.Store) *controller.Impl {
	// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
	ctxFunc := func(ctx context.Context) context.Context {
		return brokers.ToContext(gcpas.ToContext(ctx))
	}

	return validation.NewAdmissionController(ctx,

		// Name of the validation webhook.
		"validation.webhook.events.cloud.google.com",

		// The path on which to serve the webhook.
		"/validation",

		// The resources to validate and default.
		types,

		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

// NewConfigValidationController creates a new admission controller to validate configuration
// maps, which will be used when applying new configmap or editing existing configmap.
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
			leaderelection.ConfigMapName(): leaderelection.NewConfigFromConfigMap,
			gcpauth.ConfigMapName():        gcpauth.NewDefaultsConfigFromConfigMap,
			broker.ConfigMapName():         broker.NewDefaultsConfigFromConfigMap,
			dataresidency.ConfigMapName():  dataresidency.NewDefaultsConfigFromConfigMap,
		},
	)
}

type conversionController func(context.Context, configmap.Watcher) *controller.Impl

func newConversionConstructor(brokers *broker.StoreSingleton, gcpas *gcpauth.StoreSingleton) conversionController {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return newConversionController(ctx, cmw, brokers.Store(ctx, cmw), gcpas.Store(ctx, cmw))
	}
}

func newConversionController(ctx context.Context, cmw configmap.Watcher, brokers *broker.Store, gcpas *gcpauth.Store) *controller.Impl {
	var (
		eventsv1alpha1_    = eventsv1alpha1.SchemeGroupVersion.Version
		eventsv1beta1_     = eventsv1beta1.SchemeGroupVersion.Version
		eventsv1_          = eventsv1.SchemeGroupVersion.Version
		messagingv1alpha1_ = messagingv1alpha1.SchemeGroupVersion.Version
		messagingv1beta1_  = messagingv1beta1.SchemeGroupVersion.Version
		inteventsv1alpha1_ = inteventsv1alpha1.SchemeGroupVersion.Version
		inteventsv1beta1_  = inteventsv1beta1.SchemeGroupVersion.Version
		inteventsv1_       = inteventsv1.SchemeGroupVersion.Version
	)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return brokers.ToContext(gcpas.ToContext(ctx))
	}

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			// events
			eventsv1.Kind("CloudAuditLogsSource"): {
				DefinitionName: events.CloudAuditLogsSourcesResource.String(),
				HubVersion:     eventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventsv1alpha1_: &eventsv1alpha1.CloudAuditLogsSource{},
					eventsv1beta1_:  &eventsv1beta1.CloudAuditLogsSource{},
					eventsv1_:       &eventsv1.CloudAuditLogsSource{},
				},
			},
			eventsv1.Kind("CloudPubSubSource"): {
				DefinitionName: events.CloudPubSubSourcesResource.String(),
				HubVersion:     eventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventsv1alpha1_: &eventsv1alpha1.CloudPubSubSource{},
					eventsv1beta1_:  &eventsv1beta1.CloudPubSubSource{},
					eventsv1_:       &eventsv1.CloudPubSubSource{},
				},
			},
			eventsv1.Kind("CloudSchedulerSource"): {
				DefinitionName: events.CloudSchedulerSourcesResource.String(),
				HubVersion:     eventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventsv1alpha1_: &eventsv1alpha1.CloudSchedulerSource{},
					eventsv1beta1_:  &eventsv1beta1.CloudSchedulerSource{},
					eventsv1_:       &eventsv1.CloudSchedulerSource{},
				},
			},
			eventsv1.Kind("CloudStorageSource"): {
				DefinitionName: events.CloudStorageSourcesResource.String(),
				HubVersion:     eventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventsv1alpha1_: &eventsv1alpha1.CloudStorageSource{},
					eventsv1beta1_:  &eventsv1beta1.CloudStorageSource{},
					eventsv1_:       &eventsv1.CloudStorageSource{},
				},
			},
			eventsv1.Kind("CloudBuildSource"): {
				DefinitionName: events.CloudBuildSourcesResource.String(),
				HubVersion:     eventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventsv1alpha1_: &eventsv1alpha1.CloudBuildSource{},
					eventsv1beta1_:  &eventsv1beta1.CloudBuildSource{},
					eventsv1_:       &eventsv1.CloudBuildSource{},
				},
			},
			// intevents
			inteventsv1.Kind("PullSubscription"): {
				DefinitionName: intevents.PullSubscriptionsResource.String(),
				HubVersion:     inteventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					inteventsv1alpha1_: &inteventsv1alpha1.PullSubscription{},
					inteventsv1beta1_:  &inteventsv1beta1.PullSubscription{},
					inteventsv1_:       &inteventsv1.PullSubscription{},
				},
			},
			inteventsv1.Kind("Topic"): {
				DefinitionName: intevents.TopicsResource.String(),
				HubVersion:     inteventsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					inteventsv1alpha1_: &inteventsv1alpha1.Topic{},
					inteventsv1beta1_:  &inteventsv1beta1.Topic{},
					inteventsv1_:       &inteventsv1.Topic{},
				},
			},
			// messaging
			messagingv1alpha1.Kind("Channel"): {
				DefinitionName: messaging.ChannelsResource.String(),
				HubVersion:     messagingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					messagingv1alpha1_: &messagingv1alpha1.Channel{},
					messagingv1beta1_:  &messagingv1beta1.Channel{},
				},
			},
		},
		ctxFunc,
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

	controllers, err := InitializeControllers(ctx)
	if err != nil {
		log.Fatal(err)
	}
	sharedmain.WebhookMainWithContext(ctx, logconfig.WebhookName(), controllers...)
}

func Controllers(
	conversionController conversionController,
	defaultingAdmissionController defaultingAdmissionController,
	validationController validationController,
) []injection.ControllerConstructor {
	return []injection.ControllerConstructor{
		certificates.NewController,
		NewConfigValidationController,
		injection.ControllerConstructor(validationController),
		injection.ControllerConstructor(defaultingAdmissionController),
		injection.ControllerConstructor(conversionController),
	}
}
