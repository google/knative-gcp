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

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pkgapisduck "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/policy"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
)

// SubjectResolver resolves policy subject from Authorizables.
type SubjectResolver struct {
	tracker         tracker.Interface
	informerFactory pkgapisduck.InformerFactory
}

// NewSubjectResolver constructs a new SubjectResolver.
func NewSubjectResolver(ctx context.Context, callback func(types.NamespacedName)) *SubjectResolver {
	ret := &SubjectResolver{}

	ret.tracker = tracker.New(callback, controller.GetTrackerLease(ctx))
	ret.informerFactory = &pkgapisduck.CachedInformerFactory{
		Delegate: &pkgapisduck.EnqueueInformerFactory{
			Delegate:     resource.Get(ctx),
			EventHandler: controller.HandleAll(ret.tracker.OnChanged),
		},
	}

	return ret
}

// ResolveFromRef resolves policy binding subject from the reference.
func (r *SubjectResolver) ResolveFromRef(ref tracker.Reference, parent interface{}) (*metav1.LabelSelector, error) {
	// If the subject's label selector presents, then directly use it to select workload.
	if ref.Selector != nil {
		return ref.Selector, nil
	}

	if err := r.tracker.TrackReference(ref, parent); err != nil {
		return nil, fmt.Errorf("failed to track subject %+v: %w", ref, err)
	}

	gvr, _ := meta.UnsafeGuessKindToResource(ref.GroupVersionKind())
	_, lister, err := r.informerFactory.Get(gvr)
	if err != nil {
		return nil, fmt.Errorf("failed to get lister for %+v: %w", gvr, err)
	}

	obj, err := lister.ByNamespace(ref.Namespace).Get(ref.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get ref %+v: %w", ref, err)
	}

	kr, ok := obj.(*duckv1alpha1.Resource)
	if !ok {
		return nil, fmt.Errorf("%+v (%T) is not a duck Resource", ref, ref)
	}

	// Parse the annotation to resolve the subject to protect.
	selector, ok := kr.Annotations[policy.AuthorizableAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("the reference is not an authorizable; expecting annotation %q", policy.AuthorizableAnnotationKey)
	}
	// Handle this special case where the object itself is already the workload to bind policy.
	if selector == policy.SelfAuthorizableAnnotationValue {
		if len(kr.GetLabels()) == 0 {
			// It's probably too dangerous to apply a policy without specifying any label selector.
			// For now, we simply disallow that.
			return nil, errors.New("the reference is self authorizable but doesn't have any labels")
		}
		return &metav1.LabelSelector{
			MatchLabels: kr.GetLabels(),
		}, nil
	}

	var l metav1.LabelSelector
	if err := json.Unmarshal([]byte(selector), &l); err != nil {
		return nil, fmt.Errorf("the reference doesn't have a valid subject in annotation %q; it must be a LabelSelector: %w", policy.AuthorizableAnnotationKey, err)
	}

	return &l, nil
}
