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
	istiopolicy "istio.io/api/security/v1beta1"
	istiotype "istio.io/api/type/v1beta1"
	istiopolicyclient "istio.io/client-go/pkg/apis/security/v1beta1"
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

	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	bindingreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/policy/v1alpha1/httppolicybinding"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/policy"

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
		Name: "subject missing",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "not-exist"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newTestPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "not-exist"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("SubjectResolvingFailure", `failed to get ref {APIVersion:duck.knative.dev/v1 Kind:KResource Namespace:testnamespace Name:not-exist Selector:nil}: kresources.duck.knative.dev "not-exist" not found`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to resolve subject from HTTPPolicyBinding: failed to get ref {APIVersion:duck.knative.dev/v1 Kind:KResource Namespace:testnamespace Name:not-exist Selector:nil}: kresources.duck.knative.dev "not-exist" not found`),
		},
		WantErr: true,
	}, {
		Name: "subject is not authorizable",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace),
			newTestPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("SubjectResolvingFailure", `the reference is not an authorizable; expecting annotation "policy.run.cloud.google.com/authorizableOn"`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to resolve subject from HTTPPolicyBinding: the reference is not an authorizable; expecting annotation "policy.run.cloud.google.com/authorizableOn"`),
		},
		WantErr: true,
	}, {
		Name: "subject is self authorizable without labels",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": "self"}),
			),
			newTestPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("SubjectResolvingFailure", `the reference is self authorizable but doesn't have any labels`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to resolve subject from HTTPPolicyBinding: the reference is self authorizable but doesn't have any labels`),
		},
		WantErr: true,
	}, {
		Name: "subject is authorizable with invalid label selector",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": "random"}),
			),
			newTestPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("SubjectResolvingFailure", `the reference doesn't have a valid subject in annotation "policy.run.cloud.google.com/authorizableOn"; it must be a LabelSelector: invalid character 'r' looking for beginning of value`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to resolve subject from HTTPPolicyBinding: the reference doesn't have a valid subject in annotation "policy.run.cloud.google.com/authorizableOn"; it must be a LabelSelector: invalid character 'r' looking for beginning of value`),
		},
		WantErr: true,
	}, {
		Name: "policy missing",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
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
				WithPolicyBindingStatusFailure("GetPolicyFailure", `httppolicy.policy.run.cloud.google.com "testpolicy" not found`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to get HTTPPolicy: httppolicy.policy.run.cloud.google.com "testpolicy" not found`),
		},
		WantErr: true,
	}, {
		Name: "create request authentication error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "requestauthentications"),
		},
		WantCreates: []runtime.Object{
			NewRequestAuthentication(kmeta.ChildName(testBindingName, "-req-authn"), testNamespace,
				withRequestAuthenticationOwner(testBindingName),
				withRequestAuthenticationTestSpec(),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("RequestAuthenticationReconcileFailure", `failed to create Istio RequestAuthentication: inducing failure for create requestauthentications`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to create Istio RequestAuthentication: inducing failure for create requestauthentications`),
		},
		WantErr: true,
	}, {
		Name: "update request authentication error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestPolicy(testPolicyName, testNamespace),
			NewRequestAuthentication(kmeta.ChildName(testBindingName, "-req-authn"), testNamespace,
				withRequestAuthenticationOwner(testBindingName),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "requestauthentications"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewRequestAuthentication(kmeta.ChildName(testBindingName, "-req-authn"), testNamespace,
				withRequestAuthenticationTestSpec(),
				withRequestAuthenticationOwner(testBindingName),
			),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("RequestAuthenticationReconcileFailure", `failed to update Istio RequestAuthentication: inducing failure for update requestauthentications`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to update Istio RequestAuthentication: inducing failure for update requestauthentications`),
		},
		WantErr: true,
	}, {
		Name: "create authorization policy error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestPolicy(testPolicyName, testNamespace),
			NewRequestAuthentication(kmeta.ChildName(testBindingName, "-req-authn"), testNamespace,
				withRequestAuthenticationTestSpec(),
				withRequestAuthenticationOwner(testBindingName),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "authorizationpolicies"),
		},
		WantCreates: []runtime.Object{
			NewAuthorizationPolicy(kmeta.ChildName(testBindingName, "-authz"), testNamespace,
				withAuthorizationPolicyTestSpec(),
				withAuthorizationPolicyOwner(testBindingName),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("AuthorizationPolicyReconcileFailure", `failed to create Istio AuthorizationPolicy: inducing failure for create authorizationpolicies`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to create Istio AuthorizationPolicy: inducing failure for create authorizationpolicies`),
		},
		WantErr: true,
	}, {
		Name: "update authorization policy error",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestPolicy(testPolicyName, testNamespace),
			NewRequestAuthentication(kmeta.ChildName(testBindingName, "-req-authn"), testNamespace,
				withRequestAuthenticationTestSpec(),
				withRequestAuthenticationOwner(testBindingName),
			),
			NewAuthorizationPolicy(kmeta.ChildName(testBindingName, "-authz"), testNamespace,
				withAuthorizationPolicyOwner(testBindingName),
			),
		},
		Key: testNamespace + "/" + testBindingName,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "authorizationpolicies"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuthorizationPolicy(kmeta.ChildName(testBindingName, "-authz"), testNamespace,
				withAuthorizationPolicyTestSpec(),
				withAuthorizationPolicyOwner(testBindingName),
			),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusInit(),
				WithPolicyBindingStatusFailure("AuthorizationPolicyReconcileFailure", `failed to update Istio AuthorizationPolicy: inducing failure for update authorizationpolicies`),
			).AsHTTPPolicyBinding(),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to update Istio AuthorizationPolicy: inducing failure for update authorizationpolicies`),
		},
		WantErr: true,
	}, {
		Name: "success",
		Objects: []runtime.Object{
			NewPolicyBinding(
				testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
			).AsHTTPPolicyBinding(),
			newSubject("subject", testNamespace,
				WithUnstructuredAnnotations(map[string]interface{}{"policy.run.cloud.google.com/authorizableOn": `{"matchLabels":{"app":"test"}}`}),
			),
			newTestPolicy(testPolicyName, testNamespace),
		},
		Key: testNamespace + "/" + testBindingName,
		WantCreates: []runtime.Object{
			NewRequestAuthentication(kmeta.ChildName(testBindingName, "-req-authn"), testNamespace,
				withRequestAuthenticationTestSpec(),
				withRequestAuthenticationOwner(testBindingName),
			),
			NewAuthorizationPolicy(kmeta.ChildName(testBindingName, "-authz"), testNamespace,
				withAuthorizationPolicyTestSpec(),
				withAuthorizationPolicyOwner(testBindingName),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPolicyBinding(testBindingName, testNamespace,
				WithPolicyBindingSubject(testSubjectGVK, "subject"),
				WithPolicyBindingPolicy(testPolicyName),
				WithPolicyBindingStatusReady(),
			).AsHTTPPolicyBinding(),
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
			subjectResolver: policy.NewSubjectResolver(ctx, func(types.NamespacedName) {}),
		}

		return bindingreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetHTTPPolicyBindingLister(), r.Recorder, r)
	}))
}

func newTestPolicy(name, namespace string) *v1alpha1.HTTPPolicy {
	p := &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	p.Spec = v1alpha1.HTTPPolicySpec{
		JWT: &v1alpha1.JWTSpec{
			Issuer:  "example.com",
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
	p.SetDefaults(context.Background())
	return p
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

func withRequestAuthenticationTestSpec() RequestAuthnOption {
	return func(ra *istiopolicyclient.RequestAuthentication) {
		ra.Spec = istiopolicy.RequestAuthentication{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			JwtRules: []*istiopolicy.JWTRule{{
				Issuer:               "example.com",
				JwksUri:              testJwksURI,
				ForwardOriginalToken: true,
				FromHeaders: []*istiopolicy.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			}},
		}
	}
}

func withRequestAuthenticationOwner(owner string) RequestAuthnOption {
	return func(ra *istiopolicyclient.RequestAuthentication) {
		trueVal := true
		ra.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "policy.run.cloud.google.com/v1alpha1",
			Kind:               "HTTPPolicyBinding",
			Name:               owner,
			UID:                "test-uid",
			Controller:         &trueVal,
			BlockOwnerDeletion: &trueVal,
		}}
	}
}

func withAuthorizationPolicyTestSpec() AuthzPolicyOption {
	return func(ap *istiopolicyclient.AuthorizationPolicy) {
		ap.Spec = istiopolicy.AuthorizationPolicy{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Action: istiopolicy.AuthorizationPolicy_ALLOW,
			Rules: []*istiopolicy.Rule{
				{
					From: []*istiopolicy.Rule_From{
						{Source: &istiopolicy.Source{RequestPrincipals: []string{"user-a@example.com"}}},
					},
					To: []*istiopolicy.Rule_To{
						{Operation: &istiopolicy.Operation{
							Hosts:   []string{"*.mysvc.svc.cluster.local"},
							Methods: []string{"GET", "POST"},
							Paths:   []string{"/operation/*", "/admin/*"},
						}},
					},
					When: []*istiopolicy.Condition{
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
					To: []*istiopolicy.Rule_To{{
						Operation: &istiopolicy.Operation{
							Hosts: []string{"*.mysvc.svc.cluster.local"},
							Paths: []string{"/public/*"},
						},
					}},
				},
			},
		}
	}
}

func withAuthorizationPolicyOwner(owner string) AuthzPolicyOption {
	return func(ap *istiopolicyclient.AuthorizationPolicy) {
		trueVal := true
		ap.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "policy.run.cloud.google.com/v1alpha1",
			Kind:               "HTTPPolicyBinding",
			Name:               owner,
			UID:                "test-uid",
			Controller:         &trueVal,
			BlockOwnerDeletion: &trueVal,
		}}
	}
}
