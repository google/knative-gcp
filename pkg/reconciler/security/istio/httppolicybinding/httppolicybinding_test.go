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
	"testing"

	istioclient "github.com/google/knative-gcp/pkg/client/istio/injection/client"
	istiosecurity "istio.io/api/security/v1beta1"
	istiotype "istio.io/api/type/v1beta1"
	istiosecurityclient "istio.io/client-go/pkg/apis/security/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/tracker"

	"github.com/google/knative-gcp/pkg/apis/security/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/security/v1alpha1/httppolicybinding"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/security"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testBindingName       = "testpolicybinding"
	testSubjectName       = "testsubject"
	testPolicyName        = "testpolicy"
	testNamespace         = "testnamespace"
	testSubjectAPIVersion = "duck.knative.dev/v1"
	testSubjectKind       = "KResource"
	testJwksURI           = "https://example.com/jwks.json"
)

func TestAllCases(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		Key:  "foo/not-found",
	}, {
		Name: "subject missing",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("not-exist"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("not-exist"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusFailure("SubjectResolvingFailure", `Failed to get ref {APIVersion:duck.knative.dev/v1 Kind:KResource Namespace:testnamespace Name:not-exist Selector:nil}: kresources.duck.knative.dev "not-exist" not found`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to reconcile HTTPPolicyBinding: Failed to get ref {APIVersion:duck.knative.dev/v1 Kind:KResource Namespace:testnamespace Name:not-exist Selector:nil}: kresources.duck.knative.dev "not-exist" not found`),
		},
		WantErr: true,
	}, {
		Name: "subject is not authorizable",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusFailure("SubjectResolvingFailure", `The reference is not an authorizable; expecting annotation "security.knative.dev/authorizableOn"`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to reconcile HTTPPolicyBinding: The reference is not an authorizable; expecting annotation "security.knative.dev/authorizableOn"`),
		},
		WantErr: true,
	}, {
		Name: "subject is self authorizable without labels",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": "self"}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusFailure("SubjectResolvingFailure", `The reference is self authorizable but doesn't have any labels`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to reconcile HTTPPolicyBinding: The reference is self authorizable but doesn't have any labels`),
		},
		WantErr: true,
	}, {
		Name: "subject is authorizable with invalid label selector",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": "random"}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusFailure("SubjectResolvingFailure", `The reference doesn't have a valid subject in annotation "security.knative.dev/authorizableOn"; it must be a LabelSelector: invalid character 'r' looking for beginning of value`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to reconcile HTTPPolicyBinding: The reference doesn't have a valid subject in annotation "security.knative.dev/authorizableOn"; it must be a LabelSelector: invalid character 'r' looking for beginning of value`),
		},
		WantErr: true,
	}, {
		Name: "policy missing",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusFailure("GetPolicyFailure", `httppolicy.security.knative.dev "testpolicy" not found`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to get HTTPPolicy: httppolicy.security.knative.dev "testpolicy" not found`),
		},
		WantErr: true,
	}, {
		Name: "create request authentication error",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "requestauthentications"),
		},
		WantCreates: []runtime.Object{
			newRequestAuthentication(testBindingName, testBindingName, testNamespace,
				withRequestAuthenticationTestSpec(),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusClassCompatible(),
				withPolicyBindingStatusFailure("RequestAuthenticationReconcileFailure", `Failed to create Istio RequestAuthentication: inducing failure for create requestauthentications`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to create Istio RequestAuthentication: inducing failure for create requestauthentications`),
		},
		WantErr: true,
	}, {
		Name: "update request authentication error",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
			newRequestAuthentication(testBindingName, testBindingName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "requestauthentications"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newRequestAuthentication(testBindingName, testBindingName, testNamespace,
				withRequestAuthenticationTestSpec(),
			),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusClassCompatible(),
				withPolicyBindingStatusFailure("RequestAuthenticationReconcileFailure", `Failed to update Istio RequestAuthentication: inducing failure for update requestauthentications`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to update Istio RequestAuthentication: inducing failure for update requestauthentications`),
		},
		WantErr: true,
	}, {
		Name: "create authorization policy error",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
			newRequestAuthentication(testBindingName, testBindingName, testNamespace,
				withRequestAuthenticationTestSpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "authorizationpolicies"),
		},
		WantCreates: []runtime.Object{
			newAuthorizationPolicy(testBindingName, testBindingName, testNamespace,
				withAuthorizationPolicyTestSpec(),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusClassCompatible(),
				withPolicyBindingStatusFailure("AuthorizationPolicyReconcileFailure", `Failed to create Istio AuthorizationPolicy: inducing failure for create authorizationpolicies`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to create Istio AuthorizationPolicy: inducing failure for create authorizationpolicies`),
		},
		WantErr: true,
	}, {
		Name: "update authorization policy error",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
			newRequestAuthentication(testBindingName, testBindingName, testNamespace,
				withRequestAuthenticationTestSpec(),
			),
			newAuthorizationPolicy(testBindingName, testBindingName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "authorizationpolicies"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newAuthorizationPolicy(testBindingName, testBindingName, testNamespace,
				withAuthorizationPolicyTestSpec(),
			),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusInit(),
				withPolicyBindingStatusClassCompatible(),
				withPolicyBindingStatusFailure("AuthorizationPolicyReconcileFailure", `Failed to update Istio AuthorizationPolicy: inducing failure for update authorizationpolicies`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Failed to update Istio AuthorizationPolicy: inducing failure for update authorizationpolicies`),
		},
		WantErr: true,
	}, {
		Name: "success",
		Objects: []runtime.Object{
			newPolicyBinding(
				testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
			),
			newSubject("subject", testNamespace,
				withSubjectAnnotations(map[string]interface{}{"security.knative.dev/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newPolicy(testPolicyName, testNamespace,
				withTestPolicySpec(),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WantCreates: []runtime.Object{
			newRequestAuthentication(testBindingName, testBindingName, testNamespace,
				withRequestAuthenticationTestSpec(),
			),
			newAuthorizationPolicy(testBindingName, testBindingName, testNamespace,
				withAuthorizationPolicyTestSpec(),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: newPolicyBinding(testBindingName, testNamespace,
				withPolicyBindingSubject("subject"),
				withPolicyBindingPolicy(testPolicyName),
				withPolicyBindingStatusReady(),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "HTTPPolicyBindingReconciled", `HTTPPolicyBinding reconciled: "testnamespace/testpolicybinding"`),
		},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		ctx = resource.WithDuck(ctx)
		r := &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			policyLister:    listers.GetHTTPPolicyLister(),
			authnLister:     listers.GetRequestAuthenticationLister(),
			authzLister:     listers.GetAuthorizationPolicyLister(),
			istioClient:     istioclient.Get(ctx),
			policyTracker:   tracker.New(func(types.NamespacedName) {}, 0),
			subjectResolver: security.NewSubjectResolver(ctx, func(types.NamespacedName) {}),
		}

		return bindingreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetHTTPPolicyBindingLister(), r.Recorder, r)
	}))
}

type bindingOption func(*v1alpha1.HTTPPolicyBinding)
type subjectOption func(*unstructured.Unstructured)
type policyOption func(*v1alpha1.HTTPPolicy)
type requestAuthnOption func(*istiosecurityclient.RequestAuthentication)
type authzPolicyOption func(*istiosecurityclient.AuthorizationPolicy)

func newPolicyBinding(name, namespace string, opts ...bindingOption) *v1alpha1.HTTPPolicyBinding {
	b := &v1alpha1.HTTPPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid",
		},
	}
	for _, opt := range opts {
		opt(b)
	}
	b.SetDefaults(context.Background())
	return b
}

func withPolicyBindingSubject(name string) bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Spec.Subject = tracker.Reference{
			APIVersion: testSubjectAPIVersion,
			Kind:       testSubjectKind,
			Name:       name,
			Namespace:  b.Namespace,
		}
	}
}

func withPolicyBindingSubjectLabels(labels map[string]string) bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Spec.Subject = tracker.Reference{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		}
	}
}

func withPolicyBindingPolicy(name string) bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Spec.Policy = duckv1.KReference{Name: name}
	}
}

func withPolicyBindingStatusInit() bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
	}
}

func withPolicyBindingStatusReady() bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
		b.Status.MarkBindingClassCompatible()
		b.Status.MarkBindingAvailable()
	}
}

func withPolicyBindingStatusClassCompatible() bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
		b.Status.MarkBindingClassCompatible()
	}
}

func withPolicyBindingStatusFailure(reason, message string) bindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
		// b.Status.MarkBindingClassCompatible()
		b.Status.MarkBindingFailure(reason, message)
	}
}

func newPolicy(name, namespace string, opts ...policyOption) *v1alpha1.HTTPPolicy {
	p := &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func withTestPolicySpec() policyOption {
	return func(p *v1alpha1.HTTPPolicy) {
		p.Spec = v1alpha1.HTTPPolicySpec{
			JWT: &v1alpha1.JWTSpec{
				JwksURI: testJwksURI,
				FromHeaders: []v1alpha1.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			},
			Rules: []v1alpha1.HTTPPolicyRuleSpec{
				{
					JWTRule: v1alpha1.JWTRule{
						Principals: []string{"user-a@example.com"},
						Claims: []v1alpha1.KeyValuesMatch{
							{Key: "iss", Values: []v1alpha1.StringMatch{{Exact: "https://example.com"}}},
							{Key: "aud", Values: []v1alpha1.StringMatch{{Suffix: ".svc.cluster.local"}}},
						},
					},
					Headers: []v1alpha1.KeyValuesMatch{
						{Key: "K-test", Values: []v1alpha1.StringMatch{{Exact: "val1"}, {Prefix: "foo-"}}},
						{Key: "K-must-present", Values: []v1alpha1.StringMatch{{Presence: true}}},
					},
					Operations: []v1alpha1.RequestOperation{
						{
							Hosts:   []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
							Methods: []string{"GET", "POST"},
							Paths:   []v1alpha1.StringMatch{{Prefix: "/operation/"}, {Prefix: "/admin/"}},
						},
					},
				},
				{
					Operations: []v1alpha1.RequestOperation{
						{
							Hosts: []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
							Paths: []v1alpha1.StringMatch{{Prefix: "/public/"}},
						},
					},
				},
			},
		}
	}
}

func newSubject(name, namespace string, opts ...subjectOption) *unstructured.Unstructured {
	s := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": testSubjectAPIVersion,
			"kind":       testSubjectKind,
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

func withSubjectAnnotations(annotations map[string]interface{}) subjectOption {
	return func(s *unstructured.Unstructured) {
		meta := s.Object["metadata"].(map[string]interface{})
		meta["annotations"] = annotations
		// s.Object["metadata"] = meta
	}
}

func withSubjectLabels(labels map[string]interface{}) subjectOption {
	return func(s *unstructured.Unstructured) {
		meta := s.Object["metadata"].(map[string]interface{})
		meta["labels"] = labels
		// s.Object["metadata"] = meta
	}
}

func newRequestAuthentication(name, parent, namespace string, opts ...requestAuthnOption) *istiosecurityclient.RequestAuthentication {
	r := &istiosecurityclient.RequestAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef(parent)},
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func withRequestAuthenticationTestSpec() requestAuthnOption {
	return func(ra *istiosecurityclient.RequestAuthentication) {
		ra.Spec = istiosecurity.RequestAuthentication{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			JwtRules: []*istiosecurity.JWTRule{{
				JwksUri:              testJwksURI,
				ForwardOriginalToken: true,
				FromHeaders: []*istiosecurity.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			}},
		}
	}
}

func newAuthorizationPolicy(name, parent, namespace string, opts ...authzPolicyOption) *istiosecurityclient.AuthorizationPolicy {
	p := &istiosecurityclient.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            testBindingName,
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef(parent)},
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func withAuthorizationPolicyTestSpec() authzPolicyOption {
	return func(ap *istiosecurityclient.AuthorizationPolicy) {
		ap.Spec = istiosecurity.AuthorizationPolicy{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Action: istiosecurity.AuthorizationPolicy_ALLOW,
			Rules: []*istiosecurity.Rule{
				{
					From: []*istiosecurity.Rule_From{
						{Source: &istiosecurity.Source{RequestPrincipals: []string{"user-a@example.com"}}},
					},
					To: []*istiosecurity.Rule_To{
						{Operation: &istiosecurity.Operation{
							Hosts:   []string{"*.mysvc.svc.cluster.local"},
							Methods: []string{"GET", "POST"},
							Paths:   []string{"/operation/*", "/admin/*"},
						}},
					},
					When: []*istiosecurity.Condition{
						{
							Key:    "request.auth.claims[iss]",
							Values: []string{"https://example.com"},
						},
						{
							Key:    "request.auth.claims[aud]",
							Values: []string{"*.svc.cluster.local"},
						},
						{
							Key:    "request.headers[K-test]",
							Values: []string{"val1", "foo-*"},
						},
						{
							Key:    "request.headers[K-must-present]",
							Values: []string{"*"},
						},
					},
				},
				{
					To: []*istiosecurity.Rule_To{{
						Operation: &istiosecurity.Operation{
							Hosts: []string{"*.mysvc.svc.cluster.local"},
							Paths: []string{"/public/*"},
						},
					}},
				},
			},
		}
	}
}

func ownerRef(name string) metav1.OwnerReference {
	trueVal := true
	return metav1.OwnerReference{
		APIVersion:         "security.knative.dev/v1alpha1",
		Kind:               "HTTPPolicyBinding",
		Name:               name,
		UID:                "test-uid",
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}
