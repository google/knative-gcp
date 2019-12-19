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

package pullsubscription

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/resolver"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/pullsubscription/resources"
	"github.com/google/knative-gcp/pkg/tracing"
)

const (
	// Component names for metrics.
	sourceComponent  = "source"
	channelComponent = "channel"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for PullSubscription resources.
type Reconciler struct {
	*pubsub.PubSubBase

	// deploymentLister index properties about deployments.
	deploymentLister appsv1listers.DeploymentLister
	// pullSubscriptionLister index properties about pullsubscriptions.
	pullSubscriptionLister listers.PullSubscriptionLister

	uriResolver *resolver.URIResolver

	receiveAdapterImage string

	loggingConfig *logging.Config
	metricsConfig *metrics.ExporterOptions
	tracingConfig *tracingconfig.Config

	// createClientFn is the function used to create the Pub/Sub client that interacts with Pub/Sub.
	// This is needed so that we can inject a mock client for UTs purposes.
	createClientFn gpubsub.CreateFn
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the PullSubscription resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}
	// Get the PullSubscription resource with this namespace/name
	original, err := r.pullSubscriptionLister.PullSubscriptions(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("PullSubscription in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	ps := original.DeepCopy()

	// Reconcile this copy of the PullSubscription and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = r.reconcile(ctx, ps)

	// If no error is returned, mark the observed generation.
	// This has to be done before updateStatus is called.
	if reconcileErr == nil {
		ps.Status.ObservedGeneration = ps.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, ps.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := r.updateFinalizers(ctx, ps); fErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update PullSubscription finalizers", zap.Error(fErr))
		r.Recorder.Eventf(ps, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for PullSubscription %q: %v", ps.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		r.Recorder.Eventf(ps, corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", ps.Name)
	}

	if equality.Semantic.DeepEqual(original.Status, ps.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, ps); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update ps status", zap.Error(uErr))
		r.Recorder.Eventf(ps, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for PullSubscription %q: %v", ps.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(ps, corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", ps.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(ps, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}

	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, ps *v1alpha1.PullSubscription) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("pullsubscription", ps)))

	ps.Status.InitializeConditions()

	if ps.DeletionTimestamp != nil {
		logging.FromContext(ctx).Desugar().Debug("Deleting Pub/Sub subscription")
		if err := r.deleteSubscription(ctx, ps); err != nil {
			ps.Status.MarkNoSubscription("SubscriptionDeleteFailed", "Failed to delete Pub/Sub subscription: %s", err.Error())
			return err
		}
		ps.Status.MarkNoSubscription("SubscriptionDeleted", "Successfully deleted Pub/Sub subscription %q", ps.Status.SubscriptionID)
		ps.Status.SubscriptionID = ""
		removeFinalizer(ps)
		return nil
	}

	// Sink is required.
	sinkURI, err := r.resolveDestination(ctx, ps.Spec.Sink, ps)
	if err != nil {
		ps.Status.MarkNoSink("InvalidSink", err.Error())
		return err
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
	}

	addFinalizer(ps)

	subscriptionID, err := r.reconcileSubscription(ctx, ps)
	if err != nil {
		ps.Status.MarkNoSubscription("SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: %s", err.Error())
		return err
	}
	ps.Status.MarkSubscribed(subscriptionID)

	_, err = r.reconcileReceiveAdapter(ctx, ps)
	if err != nil {
		ps.Status.MarkNotDeployed("AdapterReconcileFailed", "Failed to reconcile Receive Adapter: %s", err.Error())
		return err
	}
	ps.Status.MarkDeployed()

	return nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, ps *v1alpha1.PullSubscription) (string, error) {
	if ps.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(ps.Spec.Project)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return "", err
		}
		// Set the projectID in the status.
		ps.Status.ProjectID = projectID
	}

	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	client, err := r.createClientFn(ctx, ps.Status.ProjectID)
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
			return "", fmt.Errorf("invalid ackDeadline: %s", err.Error())
		}
		subConfig.AckDeadline = ackDeadline
	}

	if ps.Spec.RetentionDuration != nil {
		retentionDuration, err := time.ParseDuration(*ps.Spec.RetentionDuration)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Invalid retentionDuration", zap.String("retentionDuration", *ps.Spec.RetentionDuration))
			return "", fmt.Errorf("invalid retentionDuration: %s", err.Error())
		}
		subConfig.RetentionDuration = retentionDuration
	}

	// If the subscription doesn't exist, create it.
	if !subExists {
		// Create a new subscription to the previous topic with the given name.
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
func (r *Reconciler) deleteSubscription(ctx context.Context, ps *v1alpha1.PullSubscription) error {
	if ps.Status.SubscriptionID == "" {
		return nil
	}

	// At this point the project ID should have been populated in the status.
	// Querying Pub/Sub as the subscription could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.createClientFn(ctx, ps.Status.ProjectID)
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

func (r *Reconciler) resolveDestination(ctx context.Context, destination duckv1.Destination, ps *v1alpha1.PullSubscription) (string, error) {
	// Setting up the namespace.
	if destination.Ref != nil {
		destination.Ref.Namespace = ps.Namespace
	}
	url, err := r.uriResolver.URIFromDestinationV1(destination, ps)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.PullSubscription) (*v1alpha1.PullSubscription, error) {
	source, err := r.pullSubscriptionLister.PullSubscriptions(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if equality.Semantic.DeepEqual(source.Status, desired.Status) {
		return source, nil
	}
	becomesReady := desired.Status.IsReady() && !source.Status.IsReady()
	// Don't modify the informers copy.
	existing := source.DeepCopy()
	existing.Status = desired.Status

	src, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		// TODO compute duration since last non-ready. See https://github.com/google/knative-gcp/issues/455.
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Desugar().Info("PullSubscription became ready", zap.Any("after", duration))
		r.Recorder.Event(source, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("PullSubscription %q became ready", source.Name))
		if metricErr := r.StatsReporter.ReportReady("PullSubscription", source.Namespace, source.Name, duration); metricErr != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to record ready for PullSubscription", zap.Error(metricErr))
		}
	}

	return src, err
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (r *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.PullSubscription) (*v1alpha1.PullSubscription, bool, error) {
	source, err := r.pullSubscriptionLister.PullSubscriptions(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := source.DeepCopy()

	var finalizers []string

	// If there's nothing to update, just return.
	existingFinalizers := sets.NewString(existing.Finalizers...)
	desiredFinalizers := sets.NewString(desired.Finalizers...)

	if desiredFinalizers.Has(finalizerName) {
		if existingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Add the finalizer.
		finalizers = append(existing.Finalizers, finalizerName)
	} else {
		if !existingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Remove the finalizer.
		existingFinalizers.Delete(finalizerName)
		finalizers = existingFinalizers.List()
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      finalizers,
			"resourceVersion": existing.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return desired, false, err
	}

	update, err := r.RunClientSet.PubsubV1alpha1().PullSubscriptions(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}

func addFinalizer(s *v1alpha1.PullSubscription) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func removeFinalizer(s *v1alpha1.PullSubscription) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) reconcileReceiveAdapter(ctx context.Context, src *v1alpha1.PullSubscription) (*appsv1.Deployment, error) {
	existing, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Desugar().Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}

	loggingConfig, err := logging.LoggingConfigToJson(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing existing logging config", zap.Error(err))
	}

	if r.metricsConfig != nil {
		component := sourceComponent
		// Set the metric component based on the channel label.
		if _, ok := src.Labels["events.cloud.google.com/channel"]; ok {
			component = channelComponent
		}
		r.metricsConfig.Component = component
	}

	metricsConfig, err := metrics.MetricsOptionsToJson(r.metricsConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing metrics config", zap.Error(err))
	}

	tracingConfig, err := tracing.ConfigToJSON(r.tracingConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing tracing config", zap.Error(err))
	}

	desired := resources.MakeReceiveAdapter(ctx, &resources.ReceiveAdapterArgs{
		Image:          r.receiveAdapterImage,
		Source:         src,
		Labels:         resources.GetLabels(controllerAgentName, src.Name),
		SubscriptionID: src.Status.SubscriptionID,
		SinkURI:        src.Status.SinkURI,
		TransformerURI: src.Status.TransformerURI,
		LoggingConfig:  loggingConfig,
		MetricsConfig:  metricsConfig,
		TracingConfig:  tracingConfig,
	})

	if existing == nil {
		ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(desired)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Error creating Receive Adapter", zap.Error(err))
			return nil, err
		}
		return ra, nil
	}
	if diff := cmp.Diff(desired.Spec, existing.Spec); diff != "" {
		existing.Spec = desired.Spec
		ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(existing)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Error updating Receive Adapter", zap.Error(err))
			return nil, err
		}
		return ra, nil
	}
	return existing, nil
}

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.PullSubscription) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(controllerAgentName, src.Name).String(),
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
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		r.Logger.Warnw("Failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.loggingConfig = logcfg
	r.Logger.Debugw("Update from logging ConfigMap", zap.Any("loggingCfg", cfg))
	// TODO: requeue all PullSubscriptions
}

func (r *Reconciler) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	// Cannot set the component here as we don't know if its a source or a channel.
	// Will set that up dynamically before creating the receive adapter.
	// Won't be able to requeue the PullSubscriptions.
	r.metricsConfig = &metrics.ExporterOptions{
		Domain:    metrics.Domain(),
		ConfigMap: cfg.Data,
	}
	r.Logger.Debugw("Update from metrics ConfigMap", zap.Any("metricsCfg", cfg))
}

func (r *Reconciler) UpdateFromTracingConfigMap(cfg *corev1.ConfigMap) {
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
	r.tracingConfig = tracingCfg
	r.Logger.Debugw("Updated Tracing config", zap.Any("tracingCfg", r.tracingConfig))
	// TODO: requeue all PullSubscriptions.
}
