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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/policy"
	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/policy/v1alpha1/eventpolicybinding"
	"github.com/google/knative-gcp/pkg/reconciler"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testBindingName = "testpolicybinding"
	testSubjectName = "testsubject"
	testPolicyName  = "testpolicy"
	testNamespace   = "testnamespace"
	testJwksURI     = "https://example.com/jwks.json"
)

var (
	testSubjectGVK = metav1.GroupVersionKind{Group: "duck.knative.dev", Version: "v1", Kind: "KResource"}
)

func TestAllCases(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		Key:  "foo/not-found",
	}, {
		Name: "policy missing",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("GetPolicyFailure", `eventpolicy.policy.run.cloud.google.com "testpolicy" not found`),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to get EventPolicy: eventpolicy.policy.run.cloud.google.com "testpolicy" not found`),
		},
		WantErr: true,
	}, {
		Name: "create http policy error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "httppolicies"),
		},
		WantCreates: []runtime.Object{
			newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("HTTPPolicyReconcileFailure", `failed to create HTTPPolicy: inducing failure for create httppolicies`),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to create HTTPPolicy: inducing failure for create httppolicies`),
		},
		WantErr: true,
	}, {
		Name: "update http policy error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
			newEmptyHTTPPolicy(testPolicyName, testNamespace, testBindingName),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "httppolicies"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("HTTPPolicyReconcileFailure", `failed to update HTTPPolicy: inducing failure for update httppolicies`),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to update HTTPPolicy: inducing failure for update httppolicies`),
		},
		WantErr: true,
	}, {
		Name: "create http policy binding error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
			newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "httppolicybindings"),
		},
		WantCreates: []runtime.Object{
			NewPolicyBinding(
				kmeta.ChildName(testBindingName, "-httpbinding"), testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(kmeta.ChildName(testPolicyName, "-http")),
				withPolicyBindingOwner(testBindingName),
			).AsHTTPPolicyBinding(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("HTTPPolicyBindingReconcileFailure", `failed to create HTTPPolicyBinding: inducing failure for create httppolicybindings`),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to create HTTPPolicyBinding: inducing failure for create httppolicybindings`),
		},
		WantErr: true,
	}, {
		Name: "update http policy binding error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
			newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
			NewPolicyBinding(
				kmeta.ChildName(testBindingName, "-httpbinding"), testNamespace,
				withPolicyBindingOwner(testBindingName),
			).AsHTTPPolicyBinding(),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "httppolicybindings"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(
				kmeta.ChildName(testBindingName, "-httpbinding"), testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(kmeta.ChildName(testPolicyName, "-http")),
				withPolicyBindingOwner(testBindingName),
			).AsHTTPPolicyBinding(),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("HTTPPolicyBindingReconcileFailure", `failed to update HTTPPolicyBinding: inducing failure for update httppolicybindings`),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to update HTTPPolicyBinding: inducing failure for update httppolicybindings`),
		},
		WantErr: true,
	}, {
		Name: "create http policy and binding success",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WantCreates: []runtime.Object{
			newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
			NewPolicyBinding(
				kmeta.ChildName(testBindingName, "-httpbinding"), testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(kmeta.ChildName(testPolicyName, "-http")),
				withPolicyBindingOwner(testBindingName),
			).AsHTTPPolicyBinding(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "EventPolicyBindingReconciled", `EventPolicyBinding reconciled: "testnamespace/testpolicybinding"`),
		},
	}, {
		Name: "http policy binding ready success",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
			newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
			NewPolicyBinding(
				kmeta.ChildName(testBindingName, "-httpbinding"), testNamespace,
				withPolicyBindingOwner(testBindingName),
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(kmeta.ChildName(testPolicyName, "-http")),
				WithPolicyBindingStatusReady(),
			).AsHTTPPolicyBinding(),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusReady(),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "EventPolicyBindingReconciled", `EventPolicyBinding reconciled: "testnamespace/testpolicybinding"`),
		},
	}, {
		Name: "http policy binding not ready",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsEventPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestEventPolicy(testPolicyName, testNamespace),
			newTestHTTPPolicy(testPolicyName, testNamespace, testBindingName),
			NewPolicyBinding(
				kmeta.ChildName(testBindingName, "-httpbinding"), testNamespace,
				withPolicyBindingOwner(testBindingName),
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(kmeta.ChildName(testPolicyName, "-http")),
				WithPolicyBindingStatusFailure("SomeReason", "HTTPPolicyBinding failed"),
			).AsHTTPPolicyBinding(),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusFailure("SomeReason", "HTTPPolicyBinding failed"),
			).AsEventPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "EventPolicyBindingReconciled", `EventPolicyBinding reconciled: "testnamespace/testpolicybinding"`),
		},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		ctx = resource.WithDuck(ctx)
		r := &Reconciler{
			Base:                    reconciler.NewBase(ctx, controllerAgentName, cmw),
			eventPolicyLister:       listers.GetEventPolicyLister(),
			httpPolicyLister:        listers.GetHTTPPolicyLister(),
			httpPolicyBindingLister: listers.GetHTTPPolicyBindingLister(),
			policyTracker:           tracker.New(func(types.NamespacedName) {}, 0),
		}

		return bindingreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetEventPolicyBindingLister(), r.Recorder, r)
	}))
}

func newTestEventPolicy(name, namespace string) *v1alpha1.EventPolicy {
	return &v1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.EventPolicySpec{
			JWT: &v1alpha1.JWTSpec{
				JwksURI: "https://example.com/jwks.json",
				Issuer:  "example.com",
				FromHeaders: []v1alpha1.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			},
			Rules: []v1alpha1.EventPolicyRuleSpec{
				{
					JWTRule: v1alpha1.JWTRule{
						Principals: []string{"user-a@example.com"},
					},
					Operations: []v1alpha1.RequestOperation{
						{
							Hosts:   []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
							Methods: []string{"GET", "POST"},
							Paths:   []v1alpha1.StringMatch{{Prefix: "/operation/"}, {Prefix: "/admin/"}},
						},
					},
					ID:          []v1alpha1.StringMatch{{Exact: "001"}},
					Source:      []v1alpha1.StringMatch{{Suffix: "example.com"}},
					Subject:     []v1alpha1.StringMatch{{Presence: true}},
					Type:        []v1alpha1.StringMatch{{Prefix: "hello"}},
					ContentType: []v1alpha1.StringMatch{{Exact: "application/json"}},
					Extensions: []v1alpha1.KeyValuesMatch{
						{Key: "custom1", Values: []v1alpha1.StringMatch{{Exact: "foo"}}},
						{Key: "custom2", Values: []v1alpha1.StringMatch{{Exact: "bar"}}},
					},
				},
			},
		},
	}
}

func newEmptyHTTPPolicy(parent, namespace, owner string) *v1alpha1.HTTPPolicy {
	trueVal := true
	return &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(parent, "-http"),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "policy.run.cloud.google.com/v1alpha1",
				Kind:               "EventPolicyBinding",
				Name:               owner,
				UID:                "test-uid",
				Controller:         &trueVal,
				BlockOwnerDeletion: &trueVal,
			}},
		},
	}
}

func newTestHTTPPolicy(parent, namespace, owner string) *v1alpha1.HTTPPolicy {
	trueVal := true
	return &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(parent, "-http"),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "policy.run.cloud.google.com/v1alpha1",
				Kind:               "EventPolicyBinding",
				Name:               owner,
				UID:                "test-uid",
				Controller:         &trueVal,
				BlockOwnerDeletion: &trueVal,
			}},
		},
		Spec: v1alpha1.HTTPPolicySpec{
			JWT: &v1alpha1.JWTSpec{
				JwksURI: "https://example.com/jwks.json",
				Issuer:  "example.com",
				FromHeaders: []v1alpha1.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			},
			Rules: []v1alpha1.HTTPPolicyRuleSpec{
				{
					JWTRule: v1alpha1.JWTRule{
						Principals: []string{"user-a@example.com"},
					},
					Operations: []v1alpha1.RequestOperation{
						{
							Hosts:   []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
							Methods: []string{"GET", "POST"},
							Paths:   []v1alpha1.StringMatch{{Prefix: "/operation/"}, {Prefix: "/admin/"}},
						},
					},
					Headers: []v1alpha1.KeyValuesMatch{
						{Key: "ce-id", Values: []v1alpha1.StringMatch{{Exact: "001"}}},
						{Key: "ce-source", Values: []v1alpha1.StringMatch{{Suffix: "example.com"}}},
						{Key: "ce-type", Values: []v1alpha1.StringMatch{{Prefix: "hello"}}},
						{Key: "ce-subject", Values: []v1alpha1.StringMatch{{Presence: true}}},
						{Key: "Content-Type", Values: []v1alpha1.StringMatch{{Exact: "application/json"}}},
						{Key: "ce-custom1", Values: []v1alpha1.StringMatch{{Exact: "foo"}}},
						{Key: "ce-custom2", Values: []v1alpha1.StringMatch{{Exact: "bar"}}},
					},
				},
			},
		},
	}
}

func newSubject(name, namespace string, opts ...UnstructuredOption) *unstructured.Unstructured {
	s := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": testSubjectGVK.Group + "/" + testSubjectGVK.Version,
			"kind":       testSubjectGVK.Kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func withPolicyBindingOwner(owner string) BindingOption {
	trueVal := true
	return func(cb *CommonBinding) {
		cb.ObjectMeta.UID = ""
		cb.ObjectMeta.Annotations = map[string]string{
			policy.PolicyBindingClassAnnotationKey: policy.IstioPolicyBindingClassValue,
		}
		cb.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "policy.run.cloud.google.com/v1alpha1",
			Kind:               "EventPolicyBinding",
			Name:               owner,
			UID:                "test-uid",
			Controller:         &trueVal,
			BlockOwnerDeletion: &trueVal,
		}}
	}
}
