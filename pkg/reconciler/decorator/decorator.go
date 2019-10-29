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

package decorator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pkgv1alpha1 "knative.dev/pkg/apis/v1alpha1"
	"knative.dev/pkg/resolver"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	servingv1listers "knative.dev/serving/pkg/client/listers/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/messaging/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/decorator/resources"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Decorators"
)

// Reconciler implements controller.Reconciler for Topic resources.
type Reconciler struct {
	*reconciler.Base

	// decoratorLister index properties about resources
	decoratorLister listers.DecoratorLister
	serviceLister   servingv1listers.ServiceLister

	uriResolver *resolver.URIResolver

	decoratorImage string
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

	// Get the Decorator resource with this namespace/name
	original, err := c.decoratorLister.Decorators(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("service %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	decorator := original.DeepCopy()

	// Reconcile this copy of the Topic and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = c.reconcile(ctx, decorator)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		decorator.Status.ObservedGeneration = decorator.Generation
	}

	if equality.Semantic.DeepEqual(original.Status, decorator.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := c.updateStatus(ctx, decorator); uErr != nil {
		logger.Warnw("Failed to update Topic status", zap.Error(uErr))
		c.Recorder.Eventf(decorator, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Decorator %q: %v", decorator.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		c.Recorder.Eventf(decorator, corev1.EventTypeNormal, "Updated", "Updated Decorator %q", decorator.GetName())
	}
	if reconcileErr != nil {
		c.Recorder.Event(decorator, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, decorator *v1alpha1.Decorator) error {
	logger := logging.FromContext(ctx)

	decorator.Status.InitializeConditions()

	if decorator.GetDeletionTimestamp() != nil {
		return nil
	}

	// Sink is required to continue.
	sinkURI, err := c.resolveDestination(ctx, decorator.Spec.Sink, decorator)
	decorator.Status.MarkSink(sinkURI)
	if err != nil {
		return err
	}

	if err := c.createOrUpdateDecorator(ctx, decorator); err != nil {
		logger.Error("Unable to create the decorator@v1alpha1", zap.Error(err))
		return err
	}

	return nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Decorator) (*v1alpha1.Decorator, error) {
	decorator, err := c.decoratorLister.Decorators(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if equality.Semantic.DeepEqual(decorator.Status, desired.Status) {
		return decorator, nil
	}
	becomesReady := desired.Status.IsReady() && !decorator.Status.IsReady()
	// Don't modify the informers copy.
	existing := decorator.DeepCopy()
	existing.Status = desired.Status

	dec, err := c.RunClientSet.MessagingV1alpha1().Decorators(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(dec.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("Decorator %q became ready after %v", decorator.Name, duration)

		if err := c.StatsReporter.ReportReady("Decorator", decorator.Namespace, decorator.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Decorator, %v", err)
		}
	}
	return dec, err
}

func (r *Reconciler) createOrUpdateDecorator(ctx context.Context, decorator *v1alpha1.Decorator) error {
	name := resources.GenerateDecoratorName(decorator)
	existing, err := r.ServingClientSet.ServingV1().Services(decorator.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Error("Unable to get an existing decorator", zap.Error(err))
			return err
		}
		existing = nil
	} else if !metav1.IsControlledBy(existing, decorator) {
		p, _ := json.Marshal(existing)
		logging.FromContext(ctx).Error("Got a preowned decorator", zap.Any("decorator", string(p)))
		return fmt.Errorf("Decorator: %s does not own Service: %s", decorator.Name, name)
	}

	desired := resources.MakeDecoratorV1alpha1(ctx, &resources.DecoratorArgs{
		Image:     r.decoratorImage,
		Decorator: decorator,
		Labels:    resources.GetLabels(controllerAgentName),
	})

	svc := existing
	if existing == nil {
		svc, err = r.ServingClientSet.ServingV1().Services(decorator.Namespace).Create(desired)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Decorator created.", zap.Error(err), zap.Any("decorator", svc))
	} else if diff := cmp.Diff(desired.Spec, existing.Spec); diff != "" {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1().Services(decorator.Namespace).Update(existing)
		if err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Info("Decorator updated.",
			zap.Error(err), zap.Any("decorator", svc), zap.String("diff", diff))
	} else {
		logging.FromContext(ctx).Desugar().Info("Reusing existing decorator", zap.Any("decorator", existing))
	}

	// Update the decorator.
	decorator.Status.PropagateServiceStatus(svc.Status.GetCondition(apis.ConditionReady))
	if svc.Status.IsReady() {
		decorator.Status.SetAddress(svc.Status.Address.URL)
	}
	return nil
}

func (c *Reconciler) resolveDestination(ctx context.Context, destination pkgv1alpha1.Destination, decorator *v1alpha1.Decorator) (string, error) {
	dest := pkgv1alpha1.Destination{
		Ref: destination.GetRef(),
		URI: destination.URI,
	}
	if dest.Ref != nil {
		dest.Ref.Namespace = decorator.Namespace
	}
	return c.uriResolver.URIFromDestination(dest, decorator)
}
