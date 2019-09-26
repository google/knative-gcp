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

package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "pubsubs.events.cloud.run"
)

// Reconciler is the controller implementation for the PubSub source.
type Reconciler struct {
	*reconciler.PubSubBase

	// pubsubLister for reading pubsubs.
	pubsubLister listers.PubSubLister
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the PubSub resource with this namespace/name
	original, err := c.pubsubLister.PubSubs(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The PubSub resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("storage '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	csr := original.DeepCopy()

	reconcileErr := c.reconcile(ctx, csr)

	if equality.Semantic.DeepEqual(original.Status, csr.Status) &&
		equality.Semantic.DeepEqual(original.ObjectMeta, csr.ObjectMeta) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(ctx, csr); err != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		c.Logger.Warn("Failed to update PubSub Source status", zap.Error(err))
		return err
	}

	if reconcileErr != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		return reconcileErr
	}

	return nil
}

func (c *Reconciler) reconcile(ctx context.Context, ps *v1alpha1.PubSub) error {
	ps.Status.ObservedGeneration = ps.Generation
	// If topic has been already configured, stash it here
	// since right below we remove them.
	topic := ps.Status.TopicID

	ps.Status.InitializeConditions()
	// And restore it.

	if topic == "" {
		topic = ps.Spec.Topic
	}

	// See if the source has been deleted.
	deletionTimestamp := ps.DeletionTimestamp

	if deletionTimestamp != nil {
		if err := c.PubSubBase.DeletePubSub(ctx, ps.Namespace, ps.Name); err != nil {
			c.Logger.Infof("Unable to delete pubsub resources : %s", err)
			return fmt.Errorf("failed to delete pubsub resources: %s", err)
		}
		c.removeFinalizer(ps)
		return nil
	}

	// Ensure that there's finalizer there, since we're about to attempt to
	// change external state with the topic, so we need to clean it up.
	err := c.ensureFinalizer(ps)
	if err != nil {
		return err
	}

	t, psubs, err := c.PubSubBase.ReconcilePubSub(ctx, ps, topic, resourceGroup)
	if err != nil {
		c.Logger.Infof("Failed to reconcile PubSub: %s", err)
		return err
	}

	c.Logger.Infof("Reconciled: PubSub: %+v PullSubscription: %+v", t, psubs)

	return nil
}

func (c *Reconciler) ensureFinalizer(ps *v1alpha1.PubSub) error {
	finalizers := sets.NewString(ps.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(ps.Finalizers, finalizerName),
			"resourceVersion": ps.ResourceVersion,
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}
	_, err = c.RunClientSet.EventsV1alpha1().PubSubs(ps.Namespace).Patch(ps.Name, types.MergePatchType, patch)
	return err

}

func (c *Reconciler) removeFinalizer(ps *v1alpha1.PubSub) error {
	// Only remove our finalizer if it's the first one.
	if len(ps.Finalizers) == 0 || ps.Finalizers[0] != finalizerName {
		return nil
	}

	// For parity with merge patch for adding, also use patch for removing
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      ps.Finalizers[1:],
			"resourceVersion": ps.ResourceVersion,
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}
	_, err = c.RunClientSet.EventsV1alpha1().PubSubs(ps.Namespace).Patch(ps.Name, types.MergePatchType, patch)
	return err
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.PubSub) (*v1alpha1.PubSub, error) {
	source, err := c.pubsubLister.PubSubs(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// Check if there is anything to update.
	if equality.Semantic.DeepEqual(source.Status, desired.Status) {
		return source, nil
	}
	becomesReady := desired.Status.IsReady() && !source.Status.IsReady()

	// Don't modify the informers copy.
	existing := source.DeepCopy()
	existing.Status = desired.Status
	src, err := c.RunClientSet.EventsV1alpha1().PubSubs(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("PubSub %q became ready after %v", source.Name, duration)

		if err := c.StatsReporter.ReportReady("PubSub", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Storage, %v", err)
		}
	}

	return src, err
}
