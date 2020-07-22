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

package pullsubscription

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/utils"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	tracingconfig "knative.dev/pkg/tracing/config"

	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	listers "github.com/google/knative-gcp/pkg/client/listers/intevents/v1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/resources"
	"github.com/google/knative-gcp/pkg/tracing"
)

const (
	// Component names for metrics.
	sourceComponent  = "source"
	channelComponent = "channel"

	deletePubSubFailedReason        = "SubscriptionDeleteFailed"
	deleteWorkloadIdentityFailed    = "WorkloadIdentityDeleteFailed"
	reconciledPubSubFailedReason    = "SubscriptionReconcileFailed"
	reconciledDataPlaneFailedReason = "DataPlaneReconcileFailed"
	reconciledSuccessReason         = "PullSubscriptionReconciled"
	workloadIdentityFailed          = "WorkloadIdentityReconcileFailed"

	// If the topic of the subscription has been deleted, the value of its topic becomes "_deleted-topic_".
	// See https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#subscription
	deletedTopic = "_deleted-topic_"
)

// Base implements the core controller logic for pullsubscription.
type Base struct {
	*intevents.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// DeploymentLister index properties about deployments.
	DeploymentLister appsv1listers.DeploymentLister
	// PullSubscriptionLister index properties about pullsubscriptions.
	PullSubscriptionLister listers.PullSubscriptionLister
	// serviceAccountLister for reading serviceAccounts.
	ServiceAccountLister corev1listers.ServiceAccountLister

	UriResolver *resolver.URIResolver

	ReceiveAdapterImage string
	ControllerAgentName string
	ResourceGroup       string

	LoggingConfig *logging.Config
	MetricsConfig *metrics.ExporterOptions
	TracingConfig *tracingconfig.Config

	// CreateClientFn is the function used to create the Pub/Sub client that interacts with Pub/Sub.
	// This is needed so that we can inject a mock client for UTs purposes.
	CreateClientFn gpubsub.CreateFn

	// ReconcileDataPlaneFn is the function used to reconcile the data plane resources.
	ReconcileDataPlaneFn ReconcileDataPlaneFunc
}

// ReconcileDataPlaneFunc is used to reconcile the data plane component(s).
type ReconcileDataPlaneFunc func(ctx context.Context, d *appsv1.Deployment, ps *v1.PullSubscription) error

func (r *Base) ReconcileKind(ctx context.Context, ps *v1.PullSubscription) reconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("pullsubscription", ps)))

	ps.Status.InitializeConditions()
	ps.Status.ObservedGeneration = ps.Generation

	// If pullsubscription doesn't have ownerReference and ServiceAccountName is provided, reconcile workload identity.
	// Otherwise, its owner will reconcile workload identity.
	if (ps.OwnerReferences == nil || len(ps.OwnerReferences) == 0) && ps.Spec.ServiceAccountName != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, ps.Spec.Project, ps); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile Pub/Sub subscription workload identity: %s", err.Error())
		}
	}

	// Sink is required.
	sinkURI, err := r.resolveDestination(ctx, ps.Spec.Sink, ps)
	if err != nil {
		ps.Status.MarkNoSink("InvalidSink", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, "InvalidSink", "InvalidSink: %s", err.Error())
	} else {
		ps.Status.MarkSink(sinkURI)
	}

	// Transformer is optional.
	if ps.Spec.Transformer != nil {
		transformerURI, err := r.resolveDestination(ctx, *ps.Spec.Transformer, ps)
		if err != nil {
			ps.Status.MarkNoTransformer("InvalidTransformer", err.Error())
		} else {
			ps.Status.MarkTransformer(transformerURI)
		}
	} else {
		// If the transformer is nil, mark is as nil and clean up the URI.
		ps.Status.MarkNoTransformer("TransformerNil", "Transformer is nil")
		ps.Status.TransformerURI = nil
	}

	subscriptionID, err := r.reconcileSubscription(ctx, ps)
	if err != nil {
		ps.Status.MarkNoSubscription(reconciledPubSubFailedReason, "Failed to reconcile Pub/Sub subscription: %s", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Failed to reconcile Pub/Sub subscription: %s", err.Error())
	}
	ps.Status.MarkSubscribed(subscriptionID)

	err = r.reconcileDataPlaneResources(ctx, ps, r.ReconcileDataPlaneFn)
	if err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledDataPlaneFailedReason, "Failed to reconcile Data Plane resource(s): %s", err.Error())
	}

	return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `PullSubscription reconciled: "%s/%s"`, ps.Namespace, ps.Name)
}

func (r *Base) reconcileSubscription(ctx context.Context, ps *v1.PullSubscription) (string, error) {
	if ps.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(ps.Spec.Project, metadataClient.NewDefaultMetadataClient())
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return "", err
		}
		// Set the projectID in the status.
		ps.Status.ProjectID = projectID
	}

	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	client, err := r.CreateClientFn(ctx, ps.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub client", zap.Error(err))
		return "", err
	}
	defer client.Close()

	// Generate the subscription name
	subID := resources.GenerateSubscriptionName(ps)

	// Load the subscription.
	sub := client.Subscription(subID)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		return "", err
	}

	t := client.Topic(ps.Spec.Topic)
	topicExists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return "", err
	}

	if !topicExists {
		return "", fmt.Errorf("Topic %q does not exist", ps.Spec.Topic)
	}

	// subConfig is the wanted config based on settings.
	subConfig := gpubsub.SubscriptionConfig{
		Topic:               t,
		RetainAckedMessages: ps.Spec.RetainAckedMessages,
	}

	if ps.Spec.AckDeadline != nil {
		ackDeadline, err := time.ParseDuration(*ps.Spec.AckDeadline)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Invalid ackDeadline", zap.String("ackDeadline", *ps.Spec.AckDeadline))
			return "", fmt.Errorf("invalid ackDeadline: %w", err)
		}
		subConfig.AckDeadline = ackDeadline
	}

	if ps.Spec.RetentionDuration != nil {
		retentionDuration, err := time.ParseDuration(*ps.Spec.RetentionDuration)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Invalid retentionDuration", zap.String("retentionDuration", *ps.Spec.RetentionDuration))
			return "", fmt.Errorf("invalid retentionDuration: %w", err)
		}
		subConfig.RetentionDuration = retentionDuration
	}

	// Check if the topic of the subscription is "_deleted-topic_"
	if subExists {
		config, err := sub.Config(ctx)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to get Pub/Sub subscription Config", zap.Error(err))
			return "", err
		}
		if config.Topic != nil && config.Topic.String() == deletedTopic {
			logging.FromContext(ctx).Desugar().Error("Detected deleted topic. Going to recreate the pull subscription. Unacked messages will be lost.")
			// Subscription with "_deleted-topic_" cannot pull from the new topic. In order to recover, we first delete
			// the sub and then create it. Unacked messages will be lost.
			if err := sub.Delete(ctx); err != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to delete the _deleted-topic_ susbscription", zap.Error(err))
				return "", fmt.Errorf("failed to delete the _deleted-topic_ susbscription: %v", err)
			}
			sub, err = client.CreateSubscription(ctx, subID, subConfig)
			if err != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to create subscription", zap.Error(err))
				return "", err
			}
		}
	} else {
		sub, err = client.CreateSubscription(ctx, subID, subConfig)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create subscription", zap.Error(err))
			return "", err
		}
	}
	// TODO update the subscription's config if needed.
	return subID, nil
}

// deleteSubscription looks at the status.SubscriptionID and if non-empty,
// hence indicating that we have created a subscription successfully
// in the PullSubscription, remove it.
func (r *Base) deleteSubscription(ctx context.Context, ps *v1.PullSubscription) error {
	if ps.Status.SubscriptionID == "" {
		return nil
	}

	// At this point the project ID should have been populated in the status.
	// Querying Pub/Sub as the subscription could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.CreateClientFn(ctx, ps.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	// Load the subscription.
	sub := client.Subscription(ps.Status.SubscriptionID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		return err
	}
	if exists {
		if err := sub.Delete(ctx); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to delete Pub/Sub subscription", zap.Error(err))
			return err
		}
	}
	return nil
}

func (r *Base) reconcileDataPlaneResources(ctx context.Context, ps *v1.PullSubscription, f ReconcileDataPlaneFunc) error {
	loggingConfig, err := logging.LoggingConfigToJson(r.LoggingConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing existing logging config", zap.Error(err))
	}

	if r.MetricsConfig != nil {
		component := sourceComponent
		// Set the metric component based on the channel label.
		if _, ok := ps.Labels["events.cloud.google.com/channel"]; ok {
			component = channelComponent
		}
		r.MetricsConfig.Component = component
	}

	metricsConfig, err := metrics.MetricsOptionsToJson(r.MetricsConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing metrics config", zap.Error(err))
	}

	tracingConfig, err := tracing.ConfigToJSON(r.TracingConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing tracing config", zap.Error(err))
	}

	desired := resources.MakeReceiveAdapter(ctx, &resources.ReceiveAdapterArgs{
		Image:            r.ReceiveAdapterImage,
		PullSubscription: ps,
		Labels:           resources.GetLabels(r.ControllerAgentName, ps.Name),
		SubscriptionID:   ps.Status.SubscriptionID,
		SinkURI:          ps.Status.SinkURI,
		TransformerURI:   ps.Status.TransformerURI,
		LoggingConfig:    loggingConfig,
		MetricsConfig:    metricsConfig,
		TracingConfig:    tracingConfig,
	})

	return f(ctx, desired, ps)
}

func (r *Base) GetOrCreateReceiveAdapter(ctx context.Context, desired *appsv1.Deployment, ps *v1.PullSubscription) (*appsv1.Deployment, error) {
	existing, err := r.getReceiveAdapter(ctx, ps)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Unable to get an existing Receive Adapter", zap.Error(err))
		return nil, err
	}
	if existing == nil {
		existing, err = r.KubeClientSet.AppsV1().Deployments(ps.Namespace).Create(desired)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Error creating Receive Adapter", zap.Error(err))
			return nil, err
		}
	}
	return existing, nil
}

func (r *Base) getReceiveAdapter(ctx context.Context, ps *v1.PullSubscription) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(ps.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(r.ControllerAgentName, ps.Name).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, ps) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Base) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		r.Logger.Warnw("Failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.LoggingConfig = logcfg
	r.Logger.Debugw("Update from logging ConfigMap", zap.Any("loggingCfg", cfg))
	// TODO: requeue all PullSubscriptions. See https://github.com/google/knative-gcp/issues/457.
}

func (r *Base) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	// Cannot set the component here as we don't know if its a source or a channel.
	// Will set that up dynamically before creating the receive adapter.
	// Won't be able to requeue the PullSubscriptions.
	r.MetricsConfig = &metrics.ExporterOptions{
		Domain:    metrics.Domain(),
		ConfigMap: cfg.Data,
	}
	r.Logger.Debugw("Update from metrics ConfigMap", zap.Any("metricsCfg", cfg))
}

func (r *Base) UpdateFromTracingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		r.Logger.Error("Tracing ConfigMap is nil")
		return
	}
	delete(cfg.Data, "_example")

	tracingCfg, err := tracingconfig.NewTracingConfigFromConfigMap(cfg)
	if err != nil {
		r.Logger.Warnw("Failed to create tracing config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.TracingConfig = tracingCfg
	r.Logger.Debugw("Updated Tracing config", zap.Any("tracingCfg", r.TracingConfig))
	// TODO: requeue all PullSubscriptions. See https://github.com/google/knative-gcp/issues/457.
}

func (r *Base) resolveDestination(ctx context.Context, destination duckv1.Destination, ps *v1.PullSubscription) (*apis.URL, error) {
	// To call URIFromDestinationV1(), dest.Ref must have a Namespace. If there is
	// no Namespace defined in dest.Ref, we will use the Namespace of the PS
	// as the Namespace of dest.Ref.
	if destination.Ref != nil && destination.Ref.Namespace == "" {
		destination.Ref.Namespace = ps.Namespace
	}
	url, err := r.UriResolver.URIFromDestinationV1(destination, ps)
	if err != nil {
		return nil, err
	}
	return url, nil
}

func (r *Base) FinalizeKind(ctx context.Context, ps *v1.PullSubscription) reconciler.Event {
	// If pullsubscription doesn't have ownerReference, and
	// k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if (ps.OwnerReferences == nil || len(ps.OwnerReferences) == 0) && ps.Spec.ServiceAccountName != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, ps.Spec.Project, ps); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete delete Pub/Sub subscription workload identity: %s", err.Error())
		}
	}

	logging.FromContext(ctx).Desugar().Debug("Deleting Pub/Sub subscription")
	if err := r.deleteSubscription(ctx, ps); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deletePubSubFailedReason, "Failed to delete Pub/Sub subscription: %s", err.Error())
	}
	return nil
}
