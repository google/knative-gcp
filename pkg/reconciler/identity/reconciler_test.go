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

package identity

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/api/iam/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeKubeClient "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS              = "test-NS"
	gServiceAccountName = "test@test"
	kServiceAccountName = "test"
	identifiableName    = "identifiable"
	projectID           = "id"
)

var (
	trueVal  = true
	falseVal = false

	identifiable = NewCloudPubSubSource(identifiableName, testNS,
		WithCloudPubSubSourceGCPServiceAccount(gServiceAccountName),
		WithCloudPubSubSourceServiceAccountName("test"))
	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	role        = "roles/iam.workloadIdentityUser"
	addbindings = []*iam.Binding{{
		Members: []string{"member1"},
		Role:    role,
	}}
	removebindings = []*iam.Binding{{
		Members: []string{"member1", "member2"},
		Role:    role,
	}}
)

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	ctx, _ := SetupFakeContext(t)

	c := NewIdentity(ctx)

	if c == nil {
		t.Fatal("Expected NewIdentity to return a non-nil value")
	}
}

func TestCreates(t *testing.T) {
	testCases := []struct {
		name                   string
		objects                []runtime.Object
		expectedServiceAccount *corev1.ServiceAccount
		wantCreates            []runtime.Object
		expectedErr            string
	}{{
		name: "k8s service account doesn't exist, create it",
		wantCreates: []runtime.Object{
			NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName),
		},
		expectedServiceAccount: NewServiceAccount("test", testNS, gServiceAccountName,
			WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
				APIVersion:         "events.cloud.google.com/v1alpha1",
				Kind:               "CloudPubSubSource",
				UID:                "test-pubsub-uid",
				Name:               identifiableName,
				Controller:         &falseVal,
				BlockOwnerDeletion: &trueVal,
			}}),
		),
		expectedErr: fmt.Sprintf("adding iam policy binding failed with: " +
			addIamPolicyBinding(context.Background(), projectID, gServiceAccountName, NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName)).Error(),
		),
	}, {
		name: "k8s service account exists, but doesn't have ownerReference",
		objects: []runtime.Object{
			NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName),
		},
		expectedServiceAccount: NewServiceAccount("test", testNS, gServiceAccountName,
			WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
				APIVersion:         "events.cloud.google.com/v1alpha1",
				Kind:               "CloudPubSubSource",
				UID:                "test-pubsub-uid",
				Name:               identifiableName,
				Controller:         &falseVal,
				BlockOwnerDeletion: &trueVal,
			}}),
		),
		expectedErr: fmt.Sprintf("adding iam policy binding failed with: " +
			addIamPolicyBinding(context.Background(), projectID, gServiceAccountName, NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName)).Error(),
		),
	}}

	defer logtesting.ClearAll()
	for _, tc := range testCases {
		cs := fakeKubeClient.NewSimpleClientset(tc.objects...)
		identity := &Identity{
			KubeClient: cs,
		}

		arl := pkgtesting.ActionRecorderList{cs}
		kserviceAccount, err := identity.ReconcileWorkloadIdentity(context.Background(), projectID, identifiable)

		if (tc.expectedErr != "" && err == nil) ||
			(tc.expectedErr == "" && err != nil) ||
			(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
			t.Errorf("Test case %q, Error mismatch, want: %q got: %q", tc.name, tc.expectedErr, err)
		}
		if diff := cmp.Diff(tc.expectedServiceAccount, kserviceAccount, ignoreLastTransitionTime); diff != "" {
			t.Errorf("Test case %q, unexpected topic (-want, +got) = %v", tc.name, diff)
		}

		// Validate creates.
		actions, err := arl.ActionsByVerb()
		for i, want := range tc.wantCreates {
			if i >= len(actions.Creates) {
				t.Errorf("Missing create: %#v", want)
				continue
			}
			got := actions.Creates[i]
			obj := got.GetObject()
			if diff := cmp.Diff(want, obj); diff != "" {
				t.Errorf("Unexpected create (-want, +got): %s", diff)
			}
		}
	}
}

func TestDeletes(t *testing.T) {
	testCases := []struct {
		name        string
		wantDeletes []clientgotesting.DeleteActionImpl
		objects     []runtime.Object
		expectedErr string
	}{{
		name: "delete k8s service account, failed with removing iam policy binding.",
		expectedErr: fmt.Sprintf("removing iam policy binding failed with: " +
			removeIamPolicyBinding(context.Background(), projectID, gServiceAccountName, NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName)).Error(),
		),
		objects: []runtime.Object{
			NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName,
				WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
					APIVersion:         "events.cloud.google.com/v1alpha1",
					Kind:               "CloudPubSubSource",
					UID:                "test-pubsub-uid",
					Name:               identifiableName,
					Controller:         &falseVal,
					BlockOwnerDeletion: &trueVal,
				}}),
			),
		},
	}, {
		name: "no need to remove k8s service account",
		objects: []runtime.Object{
			NewServiceAccount(kServiceAccountName, testNS, gServiceAccountName,
				WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
					APIVersion:         "events.cloud.google.com/v1alpha1",
					Kind:               "CloudPubSubSource",
					UID:                "test-pubsub-uid1",
					Name:               identifiableName,
					Controller:         &falseVal,
					BlockOwnerDeletion: &trueVal,
				}, {
					APIVersion:         "events.cloud.google.com/v1alpha1",
					Kind:               "CloudPubSubSource",
					UID:                "test-pubsub-uid2",
					Name:               identifiableName + "new",
					Controller:         &falseVal,
					BlockOwnerDeletion: &trueVal,
				}}),
			),
		},
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		cs := fakeKubeClient.NewSimpleClientset(tc.objects...)
		identity := &Identity{
			KubeClient: cs,
		}

		arl := pkgtesting.ActionRecorderList{cs}
		err := identity.DeleteWorkloadIdentity(context.Background(), projectID, identifiable)

		if (tc.expectedErr != "" && err == nil) ||
			(tc.expectedErr == "" && err != nil) ||
			(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
			t.Errorf("Error mismatch, want: %q got: %q", tc.expectedErr, err)
		}

		// validate deletes
		actions, err := arl.ActionsByVerb()
		if err != nil {
			t.Errorf("Error capturing actions by verb: %q", err)
		}
		for i, want := range tc.wantDeletes {
			if i >= len(actions.Deletes) {
				t.Errorf("Missing delete: %#v", want)
				continue
			}
			got := actions.Deletes[i]
			if got.GetName() != want.GetName() {
				t.Errorf("Unexpected delete[%d]: %#v", i, got)
			}
			if got.GetResource() != want.GetResource() {
				t.Errorf("Unexpected delete[%d]: %#v wanted: %#v", i, got, want)
			}
		}
		if got, want := len(actions.Deletes), len(tc.wantDeletes); got > want {
			for _, extra := range actions.Deletes[want:] {
				t.Errorf("Extra delete: %s/%s", extra.GetNamespace(), extra.GetName())
			}
		}
	}
}

func TestMakeSetIamPolicyRequest(t *testing.T) {
	testCases := []struct {
		name string
		want *iam.SetIamPolicyRequest
		got  *iam.SetIamPolicyRequest
	}{{
		name: "Add iam policy binding",
		want: &iam.SetIamPolicyRequest{
			Policy: &iam.Policy{
				Bindings: []*iam.Binding{{
					Members: []string{"member1"},
					Role:    role,
				}, {
					Members: []string{"member2"},
					Role:    role,
				}},
			},
		},
		got: makeSetIamPolicyRequest(addbindings, "add", "member2"),
	}, {
		name: "Remove iam policy binding",
		want: &iam.SetIamPolicyRequest{
			Policy: &iam.Policy{
				Bindings: []*iam.Binding{{
					Members: []string{"member1"},
					Role:    role,
				}},
			},
		},
		got: makeSetIamPolicyRequest(removebindings, "remove", "member2"),
	}, {
		name: "invalid iam policy binding action",
		want: nil,
		got:  makeSetIamPolicyRequest(removebindings, "plus", "member2"),
	}}

	for _, tc := range testCases {
		if diff := cmp.Diff(tc.want, tc.got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
		}
	}
}

func TestOwnerReferenceExists(t *testing.T) {
	source := &v1alpha1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduler-name",
			Namespace: "scheduler-namespace",
			UID:       "scheduler-uid",
		},
		Spec: v1alpha1.CloudSchedulerSourceSpec{
			PubSubSpec: duckv1alpha1.PubSubSpec{
				Project: "project-123",
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "eventing-secret-name",
					},
					Key: "eventing-secret-key",
				},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "Kitchen",
							Name:       "sink",
						},
					},
				},
			},
		},
	}
	kServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      kServiceAccountName,
			Annotations: map[string]string{
				"iam.gke.io/gcp-service-account": gServiceAccountName,
			},
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(source)},
		},
	}
	ownerReference := metav1.OwnerReference{
		APIVersion:         source.APIVersion,
		Kind:               source.Kind,
		Name:               source.Name,
		UID:                "",
		Controller:         nil,
		BlockOwnerDeletion: nil,
	}

	want := true
	got := ownerReferenceExists(kServiceAccount, ownerReference)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
