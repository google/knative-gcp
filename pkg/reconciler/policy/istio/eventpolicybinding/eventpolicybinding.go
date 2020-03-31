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

package eventpolicybinding

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/policy/v1alpha1/eventpolicybinding"
	policylisters "github.com/google/knative-gcp/pkg/client/listers/policy/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/policy/istio/eventpolicybinding/resources"
)

// Reconciler reconciles the EventPolicyBinding.
type Reconciler struct {
	*reconciler.Base

	eventPolicyLister       policylisters.EventPolicyLister
	httpPolicyLister        policylisters.HTTPPolicyLister
	httpPolicyBindingLister policylisters.HTTPPolicyBindingLister

	policyTracker tracker.Interface
}

var _ bindingreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind reconciles the EventPolicyBinding.
func (r *Reconciler) ReconcileKind(ctx context.Context, b *v1alpha1.EventPolicyBinding) pkgreconciler.Event {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("EventPolicyBinding", b))
	b.Status.InitializeConditions()
	b.Status.ObservedGeneration = b.Generation

	p, err := r.eventPolicyLister.EventPolicies(b.Spec.Policy.Namespace).Get(b.Spec.Policy.Name)
	if err != nil {
		logging.FromContext(ctx).Error("Problem getting EventPolicy", zap.Any("EventPolicy", b.Spec.Policy), zap.Error(err))
		b.Status.MarkBindingFailure("GetPolicyFailure", "%v", err)
		return fmt.Errorf("failed to get EventPolicy: %w", err)
	}
	// Track referenced policy.
	if err := r.policyTracker.TrackReference(tracker.Reference{
		APIVersion: eventPolicyGVK.GroupVersion().String(),
		Kind:       eventPolicyGVK.Kind,
		Namespace:  b.Spec.Policy.Namespace,
		Name:       b.Spec.Policy.Name,
	}, b); err != nil {
		logging.FromContext(ctx).Error("Problem tracking EventPolicy", zap.Any("EventPolicy", b.Spec.Policy), zap.Error(err))
		b.Status.MarkBindingFailure("TrackPolicyFailure", "%v", err)
		return fmt.Errorf("failed to track EventPolicy: %w", err)
	}

	hp, err := r.reconcileHTTPPolicy(ctx, b, p)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling HTTPPolicy", zap.Error(err))
		b.Status.MarkBindingFailure("HTTPPolicyReconcileFailure", "%v", err)
		return err
	}

	hpb, err := r.reconcileHTTPPolicyBinding(ctx, b, hp)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling HTTPPolicyBinding", zap.Error(err))
		b.Status.MarkBindingFailure("HTTPPolicyBindingReconcileFailure", "%v", err)
		return err
	}

	b.Status.PropagateBindingStatus(&hpb.Status)
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "EventPolicyBindingReconciled", "EventPolicyBinding reconciled: \"%s/%s\"", b.Namespace, b.Name)
}

func (r *Reconciler) reconcileHTTPPolicyBinding(
	ctx context.Context,
	b *v1alpha1.EventPolicyBinding,
	p *v1alpha1.HTTPPolicy) (*v1alpha1.HTTPPolicyBinding, error) {

	desired := resources.MakeHTTPPolicyBinding(b, p)
	existing, err := r.httpPolicyBindingLister.HTTPPolicyBindings(desired.Namespace).Get(desired.Name)
	if apierrs.IsNotFound(err) {
		existing, err = r.RunClientSet.PolicyV1alpha1().HTTPPolicyBindings(desired.Namespace).Create(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTPPolicyBinding: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get HTTPPolicyBinding: %w", err)
	}

	if !equality.Semantic.DeepEqual(desired.Spec, existing.Spec) {
		// Don't modify the informers copy.
		cp := existing.DeepCopy()
		cp.Spec = desired.Spec
		existing, err = r.RunClientSet.PolicyV1alpha1().HTTPPolicyBindings(cp.Namespace).Update(cp)
		if err != nil {
			return nil, fmt.Errorf("failed to update HTTPPolicyBinding: %w", err)
		}
	}

	return existing, nil
}

func (r *Reconciler) reconcileHTTPPolicy(
	ctx context.Context,
	b *v1alpha1.EventPolicyBinding,
	p *v1alpha1.EventPolicy) (*v1alpha1.HTTPPolicy, error) {

	desired := resources.MakeHTTPPolicy(b, p)
	existing, err := r.httpPolicyLister.HTTPPolicies(desired.Namespace).Get(desired.Name)
	if apierrs.IsNotFound(err) {
		existing, err = r.RunClientSet.PolicyV1alpha1().HTTPPolicies(desired.Namespace).Create(desired)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTPPolicy: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get HTTPPolicy: %w", err)
	}

	if !equality.Semantic.DeepEqual(desired.Spec, existing.Spec) {
		// Don't modify the informers copy.
		cp := existing.DeepCopy()
		cp.Spec = desired.Spec
		existing, err = r.RunClientSet.PolicyV1alpha1().HTTPPolicies(cp.Namespace).Update(cp)
		if err != nil {
			return nil, fmt.Errorf("failed to update HTTPPolicy: %w", err)
		}
	}

	return existing, nil
}
