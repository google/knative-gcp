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

package httppolicybinding

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/policy/v1alpha1/httppolicybinding"
	istioclientset "github.com/google/knative-gcp/pkg/client/istio/clientset/versioned"
	istiolisters "github.com/google/knative-gcp/pkg/client/istio/listers/security/v1beta1"
	policylisters "github.com/google/knative-gcp/pkg/client/listers/policy/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/policy"
	"github.com/google/knative-gcp/pkg/reconciler/policy/istio/httppolicybinding/resources"
)

// Reconciler reconciles the HTTPPolicyBinding.
type Reconciler struct {
	*reconciler.Base

	policyLister policylisters.HTTPPolicyLister
	authzLister  istiolisters.AuthorizationPolicyLister
	authnLister  istiolisters.RequestAuthenticationLister

	istioClient istioclientset.Interface

	subjectResolver *policy.SubjectResolver
	policyTracker   tracker.Interface
}

var _ bindingreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind reconciles the HTTPPolicyBinding.
func (r *Reconciler) ReconcileKind(ctx context.Context, b *v1alpha1.HTTPPolicyBinding) pkgreconciler.Event {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("HTTPPolicyBinding", b))
	b.Status.InitializeConditions()
	b.Status.ObservedGeneration = b.Generation

	subjectSelector, err := r.subjectResolver.ResolveFromRef(b.Spec.Subject, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem resolving binding subject", zap.Error(err))
		b.Status.MarkBindingFailure("SubjectResolvingFailure", "%v", err)
		return fmt.Errorf("failed to resolve subject from HTTPPolicyBinding: %w", err)
	}
	if len(subjectSelector.MatchLabels) == 0 {
		logging.FromContext(ctx).Error("Binding subject has zero label selector")
		b.Status.MarkBindingFailure("InvalidResolvedSubject", "Resolved binding subject has zero label selector")
		return errors.New("binding subject has zero label selector")
	}

	p, err := r.policyLister.HTTPPolicies(b.Spec.Policy.Namespace).Get(b.Spec.Policy.Name)
	if err != nil {
		logging.FromContext(ctx).Error("Problem getting HTTPPolicy", zap.Any("HTTPPolicy", b.Spec.Policy), zap.Error(err))
		b.Status.MarkBindingFailure("GetPolicyFailure", "%v", err)
		return fmt.Errorf("failed to get HTTPPolicy: %w", err)
	}
	// Track referenced policy.
	if err := r.policyTracker.TrackReference(tracker.Reference{
		APIVersion: httpPolicyGVK.GroupVersion().String(),
		Kind:       httpPolicyGVK.Kind,
		Namespace:  b.Spec.Policy.Namespace,
		Name:       b.Spec.Policy.Name,
	}, b); err != nil {
		logging.FromContext(ctx).Error("Problem tracking HTTPPolicy", zap.Any("HTTPPolicy", b.Spec.Policy), zap.Error(err))
		b.Status.MarkBindingFailure("TrackPolicyFailure", "%v", err)
		return fmt.Errorf("failed to track HTTPPolicy: %w", err)
	}

	if err := r.reconcileRequestAuthentication(ctx, b, subjectSelector, p); err != nil {
		logging.FromContext(ctx).Error("Problem reconciling Istio RequestAuthentication", zap.Error(err))
		b.Status.MarkBindingFailure("RequestAuthenticationReconcileFailure", "%v", err)
		return err
	}

	if err := r.reconcileAuthorizationPolicy(ctx, b, subjectSelector, p); err != nil {
		logging.FromContext(ctx).Error("Problem reconciling Istio AuthorizationPolicy", zap.Error(err))
		b.Status.MarkBindingFailure("AuthorizationPolicyReconcileFailure", "%v", err)
		return err
	}

	b.Status.MarkBindingAvailable()
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "HTTPPolicyBindingReconciled", "HTTPPolicyBinding reconciled: \"%s/%s\"", b.Namespace, b.Name)
}

func (r *Reconciler) reconcileRequestAuthentication(
	ctx context.Context,
	b *v1alpha1.HTTPPolicyBinding,
	subjectSelector *metav1.LabelSelector,
	p *v1alpha1.HTTPPolicy) error {

	// JWT spec missing means no authentication is required.
	if p.Spec.JWT == nil {
		return nil
	}

	desired := resources.MakeRequestAuthentication(b, subjectSelector, *p.Spec.JWT)

	existing, err := r.authnLister.RequestAuthentications(desired.Namespace).Get(desired.Name)
	if apierrs.IsNotFound(err) {
		existing, err = r.istioClient.SecurityV1beta1().RequestAuthentications(desired.Namespace).Create(&desired)
		if err != nil {
			return fmt.Errorf("failed to create Istio RequestAuthentication: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get Istio RequestAuthentication: %w", err)
	}

	if !equality.Semantic.DeepEqual(desired.Spec, existing.Spec) {
		// Don't modify the informers copy.
		cp := existing.DeepCopy()
		cp.Spec = desired.Spec
		existing, err = r.istioClient.SecurityV1beta1().RequestAuthentications(cp.Namespace).Update(cp)
		if err != nil {
			return fmt.Errorf("failed to update Istio RequestAuthentication: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) reconcileAuthorizationPolicy(
	ctx context.Context,
	b *v1alpha1.HTTPPolicyBinding,
	subjectSelector *metav1.LabelSelector,
	p *v1alpha1.HTTPPolicy) error {

	// Rules missing means no authorization is required.
	if len(p.Spec.Rules) == 0 {
		return nil
	}

	desired := resources.MakeAuthorizationPolicy(b, subjectSelector, p.Spec.Rules)

	existing, err := r.authzLister.AuthorizationPolicies(desired.Namespace).Get(desired.Name)
	if apierrs.IsNotFound(err) {
		existing, err = r.istioClient.SecurityV1beta1().AuthorizationPolicies(desired.Namespace).Create(&desired)
		if err != nil {
			return fmt.Errorf("failed to create Istio AuthorizationPolicy: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get Istio AuthorizationPolicy: %w", err)
	}

	if !equality.Semantic.DeepEqual(desired.Spec, existing.Spec) {
		// Don't modify the informers copy.
		cp := existing.DeepCopy()
		cp.Spec = desired.Spec
		existing, err = r.istioClient.SecurityV1beta1().AuthorizationPolicies(cp.Namespace).Update(cp)
		if err != nil {
			return fmt.Errorf("failed to update Istio AuthorizationPolicy: %w", err)
		}
	}

	return nil
}
