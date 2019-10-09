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
	"time"

	"knative.dev/pkg/metrics"

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

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/duck"
	ops "github.com/google/knative-gcp/pkg/operations"
	pubsubOps "github.com/google/knative-gcp/pkg/operations/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pullsubscription/resources"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "PullSubscriptions"

	// Component names for metrics.
	sourceComponent  = "source"
	channelComponent = "channel"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for PullSubscription resources.
type Reconciler struct {
	*pubsub.PubSubBase

	deploymentLister appsv1listers.DeploymentLister

	// listers index properties about resources
	sourceLister listers.PullSubscriptionLister

	tracker tracker.Interface // TODO: use tracker for sink.

	receiveAdapterImage string

	loggingConfig *logging.Config
	metricsConfig *metrics.ExporterOptions

	//	eventTypeReconciler eventtype.Reconciler // TODO: event types.
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Service resource
// with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	logger := logging.FromContext(ctx)

	// Get the PullSubscription resource with this namespace/name
	original, err := c.sourceLister.PullSubscriptions(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("service %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	source := original.DeepCopy()

	// Reconcile this copy of the source and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = c.reconcile(ctx, source)

	// If no error is returned, mark the observed generation.
	// This has to be done before updateStatus is called.
	if reconcileErr == nil {
		source.Status.ObservedGeneration = source.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, source.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := c.updateFinalizers(ctx, source); fErr != nil {
		logger.Warnw("Failed to update PullSubscription finalizers", zap.Error(fErr))
		c.Recorder.Eventf(source, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for PullSubscription %q: %v", source.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		c.Recorder.Eventf(source, corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", source.GetName())
	}

	if equality.Semantic.DeepEqual(original.Status, source.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := c.updateStatus(ctx, source); uErr != nil {
		logger.Warnw("Failed to update source status", zap.Error(uErr))
		c.Recorder.Eventf(source, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for PullSubscription %q: %v", source.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		c.Recorder.Eventf(source, corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", source.GetName())
	}
	if reconcileErr != nil {
		c.Recorder.Event(source, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, source *v1alpha1.PullSubscription) error {
	logger := logging.FromContext(ctx)

	source.Status.ObservedGeneration = source.Generation
	source.Status.InitializeConditions()

	if source.GetDeletionTimestamp() != nil {
		logger.Info("Source Deleting.")

		state, err := c.EnsureSubscriptionDeleted(ctx, source, *source.Spec.Secret, source.Spec.Project, source.Spec.Topic, source.Status.SubscriptionID)
		switch state {
		case ops.OpsJobGetFailed:
			logger.Error("Failed to get subscription ops job.", zap.Any("state", state), zap.Error(err))
			return err

		case ops.OpsJobCreated:
			// If we created a job to make a subscription, then add the finalizer and update the status.
			source.Status.MarkSubscriptionOperation(
				"Deleting",
				"Created Job to delete Subscription %q.",
				source.Status.SubscriptionID)
			return nil

		case ops.OpsJobCompleteSuccessful:
			source.Status.MarkNoSubscription(
				"Deleted",
				"Successfully deleted Subscription %q.",
				source.Status.SubscriptionID)
			source.Status.SubscriptionID = ""
			removeFinalizer(source)

		case ops.OpsJobCreateFailed, ops.OpsJobCompleteFailed:
			logger.Error("Failed to delete subscription.", zap.Any("state", state), zap.Error(err))

			msg := "unknown"
			if err != nil {
				msg = err.Error()
			}
			source.Status.MarkNoSubscription(
				"DeleteFailed",
				"Failed to delete Subscription: %q",
				msg)
			return err
		}

		return nil
	}

	// Sink is required.
	sinkURI, err := c.resolveDestination(ctx, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("InvalidSink", err.Error())
		return err
	} else {
		source.Status.MarkSink(sinkURI.String())
	}

	// Transformer is optional.
	if source.Spec.Transformer != nil {
		transformerURI, err := c.resolveDestination(ctx, *source.Spec.Transformer, source.Namespace)
		if err != nil {
			source.Status.MarkNoTransformer("InvalidTransformer", err.Error())
		} else {
			source.Status.MarkTransformer(transformerURI.String())
		}
	}

	source.Status.SubscriptionID = resources.GenerateSubscriptionName(source)

	state, err := c.EnsureSubscriptionCreated(ctx, source, *source.Spec.Secret, source.Spec.Project, source.Spec.Topic,
		source.Status.SubscriptionID, source.Spec.GetAckDeadline(), source.Spec.RetainAckedMessages, source.Spec.GetRetentionDuration())
	switch state {
	case ops.OpsJobGetFailed:
		logger.Error("Failed to get subscription ops job.", zap.Any("state", state), zap.Error(err))
		return err

	case ops.OpsJobCreated:
		// If we created a job to make a subscription, then add the finalizer and update the status.
		addFinalizer(source)
		source.Status.MarkSubscriptionOperation("Creating",
			"Created Job to create Subscription %q.",
			source.Status.SubscriptionID)
		return nil

	case ops.OpsJobCompleteSuccessful:
		source.Status.MarkSubscribed()

		// Now that the job completed, grab the pod status for it.
		// Note that since it's not yet currently relied upon, don't hard
		// fail this if we can't fetch it. Just warn
		jobName := pubsubOps.SubscriptionJobName(source, "create")
		logger.Info("Finding job pods for", zap.String("jobName", jobName))
		jobPod, err := ops.GetJobPodByJobName(ctx, c.KubeClientSet, source.Namespace, jobName)
		if err != nil {
			logger.Error("Failed to fetch job pods, can not fetch projectid",
				zap.Error(err))
		}
		if jobPod != nil {
			terminationMessage := ops.GetFirstTerminationMessage(jobPod)
			if terminationMessage != "" {
				var sar pubsubOps.SubActionResult
				err = json.Unmarshal([]byte(terminationMessage), &sar)
				if err == nil {
					c.Logger.Infof("Topic project: %q", sar.ProjectId)
					source.Status.ProjectID = sar.ProjectId
				}
			}
		}

	case ops.OpsJobCreateFailed, ops.OpsJobCompleteFailed:
		logger.Error("Failed to create subscription.", zap.Any("state", state), zap.Error(err))

		msg := "unknown"
		if err != nil {
			msg = err.Error()
		}
		source.Status.MarkNoSubscription(
			"CreateFailed",
			"Failed to create Subscription: %q.",
			msg)
		return err
	}

	_, err = c.createOrUpdateReceiveAdapter(ctx, source)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	source.Status.MarkDeployed()

	// TODO: Registry
	//// Only create EventTypes for Broker sinks.
	//if source.Spec.Sink.Kind == "Broker" {
	//	err = r.reconcileEventTypes(ctx, src)
	//	if err != nil {
	//		logger.Error("Unable to reconcile the event types", zap.Error(err))
	//		return err
	//	}
	//	src.Status.MarkEventTypes()
	//}

	return nil
}

func (c *Reconciler) resolveDestination(ctx context.Context, destination apisv1alpha1.Destination, namespace string) (*apis.URL, error) {
	if destination.URI != nil {
		return destination.URI, nil
	} else {
		if uri, err := duck.GetSinkURI(ctx, c.DynamicClientSet, destination.ObjectReference, namespace); err != nil {
			return nil, err
		} else {
			if destURI, err := apis.ParseURL(uri); err != nil {
				return nil, err
			} else {
				return destURI, nil
			}
		}
	}
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.PullSubscription) (*v1alpha1.PullSubscription, error) {
	source, err := c.sourceLister.PullSubscriptions(desired.Namespace).Get(desired.Name)
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

	src, err := c.RunClientSet.PubsubV1alpha1().PullSubscriptions(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("PullSubscription %q became ready after %v", source.Name, duration)

		if err := c.StatsReporter.ReportReady("PullSubscription", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for PullSubscription, %v", err)
		}
	}

	return src, err
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (c *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.PullSubscription) (*v1alpha1.PullSubscription, bool, error) {
	source, err := c.sourceLister.PullSubscriptions(desired.Namespace).Get(desired.Name)
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

	update, err := c.RunClientSet.PubsubV1alpha1().PullSubscriptions(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
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

func (r *Reconciler) createOrUpdateReceiveAdapter(ctx context.Context, src *v1alpha1.PullSubscription) (*appsv1.Deployment, error) {
	existing, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}

	loggingConfig, err := logging.LoggingConfigToJson(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Error("Error serializing existing logging config", zap.Error(err))
	}

	if r.metricsConfig != nil {
		component := sourceComponent
		// Set the metric component based on PullSubscription label.
		if _, ok := src.Labels["events.cloud.google.com/channel"]; ok {
			component = channelComponent
		}
		r.metricsConfig.Component = component
	}

	metricsConfig, err := metrics.MetricsOptionsToJson(r.metricsConfig)
	if err != nil {
		logging.FromContext(ctx).Error("Error serializing metrics config", zap.Error(err))
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
	})

	if existing == nil {
		ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(desired)
		logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Error(err), zap.Any("receiveAdapter", ra))
		return ra, err
	}
	if diff := cmp.Diff(desired.Spec, existing.Spec); diff != "" {
		existing.Spec = desired.Spec
		ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(existing)
		logging.FromContext(ctx).Desugar().Info("Receive Adapter updated.",
			zap.Error(err), zap.Any("receiveAdapter", ra), zap.String("diff", diff))
		return ra, err
	}
	logging.FromContext(ctx).Desugar().Info("Reusing existing Receive Adapter", zap.Any("receiveAdapter", existing))
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
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments: %v", zap.Error(err))
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
		r.Logger.Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.loggingConfig = logcfg
	r.Logger.Infow("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
	// TODO: requeue all pullsubscriptions
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
	r.Logger.Infow("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
}

// TODO: Registry
//func (r *Reconciler) reconcileEventTypes(ctx context.Context, src *v1alpha1.PullSubscription) error {
//	args := r.newEventTypeReconcilerArgs(src)
//	return r.eventTypeReconciler.Reconcile(ctx, src, args)
//}
//
//func (r *Reconciler) newEventTypeReconcilerArgs(src *v1alpha1.PubSub) *eventtype.ReconcilerArgs {
//	spec := eventingv1alpha1.EventTypeSpec{
//		Type:   v1alpha1.PubSubEventType,
//		Source: v1alpha1.GetPubSub(src.Status.ProjectID, src.Spec.Topic),
//		Broker: src.Spec.Sink.Name,
//	}
//	specs := make([]eventingv1alpha1.EventTypeSpec, 0, 1)
//	specs = append(specs, spec)
//	return &eventtype.ReconcilerArgs{
//		Specs:     specs,
//		Namespace: src.Namespace,
//		Labels:    getLabels(src),
//	}
//}
