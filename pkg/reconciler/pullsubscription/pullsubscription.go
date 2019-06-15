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
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	listers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/duck"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pullsubscription/resources"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "PullSubscriptions"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for PullSubscription resources.
type Reconciler struct {
	*reconciler.Base

	deploymentLister appsv1listers.DeploymentLister

	// listers index properties about resources
	sourceLister listers.PullSubscriptionLister

	tracker tracker.Interface // TODO: use tracker for sink.

	receiveAdapterImage  string
	subscriptionOpsImage string

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

	// Get the Service resource with this namespace/name
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

	if equality.Semantic.DeepEqual(original.Finalizers, source.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := c.updateFinalizers(ctx, source); fErr != nil {
		logger.Warnw("Failed to update source finalizers", zap.Error(fErr))
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

	source.Status.InitializeConditions()

	if source.GetDeletionTimestamp() != nil {
		logger.Info("Source Deleting.")
		err := c.ensureSubscriptionRemoval(ctx, source)
		if err != nil {
			logger.Error("Unable to delete the Subscription", zap.Error(err))
			return err
		}
		return nil
	}

	sinkURI, err := duck.GetSinkURI(ctx, c.DynamicClientSet, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	source.Status.MarkSink(sinkURI)

	var transformerURI string
	if source.Spec.Transformer != nil {
		transformerURI, err = duck.GetSinkURI(ctx, c.DynamicClientSet, source.Spec.Transformer, source.Namespace)
		if err != nil {
			source.Status.MarkNoSink("NotFound", "")
			return err
		}
		source.Status.MarkSink(sinkURI)
	}

	subID := resources.GenerateSubName(source)

	cont, err := c.ensureSubscription(ctx, source, subID)
	if err != nil {
		logger.Error("Unable to ensure subscription", zap.Error(err))
		return err
	}
	if !cont {
		logger.Info("Waiting for Job.")
		return nil
	}

	_, err = c.createReceiveAdapter(ctx, source, subID, sinkURI, transformerURI)
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

	source.Status.ObservedGeneration = source.Generation

	return nil
}

func (c *Reconciler) ensureSubscription(ctx context.Context, source *v1alpha1.PullSubscription, subID string) (bool, error) {
	if source.Status.GetCondition(v1alpha1.PullSubscriptionConditionSubscribed).IsTrue() {
		c.Logger.Info("Subscription is subscribed.")
		// For now, we will trust the status, no healing.
		return true, nil
	}

	job, err := c.getJob(ctx, source, labels.SelectorFromSet(resources.JobLabels(source.Name, resources.ActionCreate)))
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Info("Job not found, creating.")

		job = resources.NewSubscriptionOps(resources.Args{
			Image:          c.subscriptionOpsImage,
			Action:         resources.ActionCreate,
			ProjectID:      source.Spec.Project,
			TopicID:        source.Spec.Topic,
			SubscriptionID: subID,
			Source:         source,
		})

		job, err := c.KubeClientSet.BatchV1().Jobs(source.GetNamespace()).Create(job)
		if err != nil {
			c.Logger.Info("Failed to create Job.", zap.Error(err))
			return false, nil
		}
		// If we created a job to make a subscription, then add the finalizer and update the status.
		if job != nil {
			addFinalizer(source)
			source.Status.MarkSubscribing("Creating", "Created Job %q to create Subscription %q.", job.Name, subID)
			source.Status.SubscriptionID = subID
		}
		c.Logger.Info("Created Job.")
		return false, nil
	} else if err != nil {
		c.Logger.Info("Failed to get Job.", zap.Error(err))
		return false, err
	}

	if resources.IsJobComplete(job) {
		c.Logger.Info("Job Complete.")
		if resources.IsJobSucceeded(job) {
			source.Status.MarkSubscribed()
			return true, nil
		} else if resources.IsJobFailed(job) {
			source.Status.MarkNotSubscribed("CreateFailed", "Failed to create Subscription: %q", resources.JobFailedMessage(job))
		}
	} else {
		c.Logger.Info("Job still active.", zap.Any("job", job))
	}
	return false, nil
}

func (c *Reconciler) ensureSubscriptionRemoval(ctx context.Context, source *v1alpha1.PullSubscription) error {
	c.Logger.Info("Starting to Ensure Subscription Removal.", zap.Any("source.name", source.Name))
	if source.Status.SubscriptionID == "" {
		c.Logger.Info("SubscriptionID is empty.")
		// For now, we will trust the status, no healing.
		return nil
	}

	job, err := c.getJob(ctx, source, labels.SelectorFromSet(resources.JobLabels(source.Name, resources.ActionDelete)))
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		job = resources.NewSubscriptionOps(resources.Args{
			Image:          c.subscriptionOpsImage,
			Action:         resources.ActionDelete,
			ProjectID:      source.Spec.Project,
			TopicID:        source.Spec.Topic,
			SubscriptionID: source.Status.SubscriptionID,
			Source:         source,
		})

		job, err := c.KubeClientSet.BatchV1().Jobs(source.GetNamespace()).Create(job)
		if err != nil {
			c.Logger.Info("Failed to create Job.", zap.Error(err))
			return nil
		}
		// If we created a job to make a subscription, then add the finalizer and update the status.
		if job != nil {
			source.Status.MarkUnsubscribing("Deleting", "Created Job %q to delete Subscription %q.", job.Name, source.Status.SubscriptionID)
		}
		return nil
	} else if err != nil {
		c.Logger.Info("Failed to get Job.", zap.Error(err))
		return err
	}

	if resources.IsJobComplete(job) {
		c.Logger.Info("Job Complete.")
		if resources.IsJobSucceeded(job) {
			source.Status.MarkUnsubscribed()
			source.Status.SubscriptionID = ""
			removeFinalizer(source)
		} else if resources.IsJobFailed(job) {
			source.Status.MarkNotSubscribed("DeleteFailed", "Failed to delete Subscription: %q",
				resources.JobFailedMessage(job))
		}
	}
	return nil
}

func (r *Reconciler) getJob(ctx context.Context, source kmeta.Accessor, ls labels.Selector) (*v1.Job, error) {
	list, err := r.KubeClientSet.BatchV1().Jobs(source.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: ls.String(),
	})
	if err != nil {
		return nil, err
	}

	for _, i := range list.Items {
		if metav1.IsControlledBy(&i, source) {
			return &i, nil
		}
	}

	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.PullSubscription) (*v1alpha1.PullSubscription, error) {
	source, err := c.sourceLister.PullSubscriptions(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(source.Status, desired.Status) {
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

func (c *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.PullSubscription) (*v1alpha1.PullSubscription, bool, error) {
	source, err := c.sourceLister.PullSubscriptions(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := source.DeepCopy()

	var finalizers []string

	// If there's nothing to update, just return.
	exisitingFinalizers := sets.NewString(existing.Finalizers...)
	desiredFinalizers := sets.NewString(desired.Finalizers...)

	if desiredFinalizers.Has(finalizerName) {
		if exisitingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Add the finalizer.
		finalizers = append(existing.Finalizers, finalizerName)
	} else {
		if !exisitingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Remove the finalizer.
		exisitingFinalizers.Delete(finalizerName)
		finalizers = exisitingFinalizers.List()
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

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.PullSubscription, subscriptionID, sinkURI, transformerURI string) (*appsv1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		return ra, nil
	}
	dp := resources.MakeReceiveAdapter(&resources.ReceiveAdapterArgs{
		Image:          r.receiveAdapterImage,
		Source:         src,
		Labels:         resources.GetLabels(controllerAgentName, src.Name),
		SubscriptionID: subscriptionID,
		SinkURI:        sinkURI,
		TransformerURI: transformerURI,
	})
	dp, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(dp)
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Error(err), zap.Any("receiveAdapter", dp))
	return dp, err
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
