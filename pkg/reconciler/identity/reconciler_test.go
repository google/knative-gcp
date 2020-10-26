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
	"errors"
	"strings"
	"testing"

	v1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeKubeClient "k8s.io/client-go/kubernetes/fake"

	clientgotesting "k8s.io/client-go/testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/client/injection/kube/client/fake"
	. "knative.dev/pkg/configmap/testing"
	"knative.dev/pkg/kmeta"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/duck"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	gclient "github.com/google/knative-gcp/pkg/gclient/iam/admin"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS              = "test-NS"
	gServiceAccountName = "test@test"
	kServiceAccountName = "test-fake-cluster-name"
	identifiableName    = "identifiable"
	projectID           = "id"
)

var (
	trueVal  = true
	falseVal = false

	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	role = "roles/iam.workloadIdentityUser"
)

func TestKSACreates(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                   string
		objects                []runtime.Object
		config                 *corev1.ConfigMap
		expectedServiceAccount *corev1.ServiceAccount
		wantCreates            []runtime.Object
		wantErrCode            codes.Code
	}{
		// Due to the limitation mentioned in https://github.com/google/knative-gcp/issues/1037,
		// skip test case "k8s service account doesn't exist, failed to get cluster name annotation."
		{
			name:   "non-default serviceAccountName, no need to create a k8s service account",
			config: ConfigMapFromTestFile(t, "config-gcp-auth-empty", "default-auth-config"),
		}, {
			name:   "default serviceAccountName, k8s service account doesn't exist, create it",
			config: ConfigMapFromTestFile(t, "config-gcp-auth", "default-auth-config"),
			wantCreates: []runtime.Object{
				NewServiceAccount(kServiceAccountName, testNS, WithServiceAccountAnnotation(gServiceAccountName)),
			},
			expectedServiceAccount: NewServiceAccount(kServiceAccountName, testNS,
				WithServiceAccountAnnotation(gServiceAccountName),
				WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
					APIVersion:         "events.cloud.google.com/v1",
					Kind:               "CloudPubSubSource",
					UID:                "test-pubsub-uid",
					Name:               identifiableName,
					Controller:         &falseVal,
					BlockOwnerDeletion: &trueVal,
				}}),
			),
			wantErrCode: codes.NotFound,
		}, {
			name: "default serviceAccountName, k8s service account exists, but doesn't have ownerReference",
			objects: []runtime.Object{
				NewServiceAccount(kServiceAccountName, testNS, WithServiceAccountAnnotation(gServiceAccountName)),
			},
			config: ConfigMapFromTestFile(t, "config-gcp-auth", "default-auth-config"),
			expectedServiceAccount: NewServiceAccount(kServiceAccountName, testNS,
				WithServiceAccountAnnotation(gServiceAccountName),
				WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
					APIVersion:         "events.cloud.google.com/v1",
					Kind:               "CloudPubSubSource",
					UID:                "test-pubsub-uid",
					Name:               identifiableName,
					Controller:         &falseVal,
					BlockOwnerDeletion: &trueVal,
				}}),
			),
			wantErrCode: codes.NotFound,
		}, {
			name: "default serviceAccountName, k8s service account exists, but doesn't have annotation",
			objects: []runtime.Object{
				NewServiceAccount(kServiceAccountName, testNS),
			},
			config: ConfigMapFromTestFile(t, "config-gcp-auth", "default-auth-config"),
			expectedServiceAccount: NewServiceAccount(kServiceAccountName, testNS,
				WithServiceAccountAnnotation(gServiceAccountName),
				WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
					APIVersion:         "events.cloud.google.com/v1",
					Kind:               "CloudPubSubSource",
					UID:                "test-pubsub-uid",
					Name:               identifiableName,
					Controller:         &falseVal,
					BlockOwnerDeletion: &trueVal,
				}}),
			),
			wantErrCode: codes.NotFound,
		}}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cs := fakeKubeClient.NewSimpleClientset(tc.objects...)
			iamClient := gclient.NewTestClient()
			m, err := iam.NewIAMPolicyManager(ctx, iamClient)
			if err != nil {
				t.Fatal(err)
			}

			identity := &Identity{
				kubeClient:    cs,
				policyManager: m,
				gcpAuthStore:  NewGCPAuthTestStore(t, tc.config),
			}
			identifiable := v1.NewCloudPubSubSource(identifiableName, testNS,
				v1.WithCloudPubSubSourceSetDefaults)
			identifiable.Spec.ServiceAccountName = kServiceAccountName
			identifiable.SetAnnotations(map[string]string{
				duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
			})

			arl := pkgtesting.ActionRecorderList{cs}
			kserviceAccount, err := identity.ReconcileWorkloadIdentity(ctx, projectID, identifiable)

			var statusErr interface{ GRPCStatus() *status.Status }
			if errors.As(err, &statusErr) {
				if code := statusErr.GRPCStatus().Code(); code != tc.wantErrCode {
					t.Fatalf("error code: want %v, got %v", tc.wantErrCode, code)
				}
			} else {
				if tc.wantErrCode != codes.OK {
					t.Fatal(err)
				}
			}
			if diff := cmp.Diff(tc.expectedServiceAccount, kserviceAccount, ignoreLastTransitionTime); diff != "" {
				t.Errorf("unexpected kserviceAccount (-want, +got) = %v", diff)
			}

			// Validate creates.
			actions, err := arl.ActionsByVerb()
			if err != nil {
				t.Fatal(err)
			}
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
		})
	}
}

func TestKSADeletes(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		wantDeletes []clientgotesting.DeleteActionImpl
		objects     []runtime.Object
		config      *corev1.ConfigMap
		wantErrCode codes.Code
	}{
		// Due to the limitation mentioned in https://github.com/google/knative-gcp/issues/1037,
		// skip test case "delete k8s service account, failed to get cluster name annotation."
		{
			name:   "non-default serviceAccountName, no need to run finalizer",
			config: ConfigMapFromTestFile(t, "config-gcp-auth-empty", "default-auth-config"),
		}, {
			name: "default serviceAccountName, delete k8s service account, failed with removing iam policy binding.",
			objects: []runtime.Object{
				NewServiceAccount(kServiceAccountName, testNS,
					WithServiceAccountAnnotation(gServiceAccountName),
					WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
						APIVersion:         "events.cloud.google.com/v1beta1",
						Kind:               "CloudPubSubSource",
						UID:                "test-pubsub-uid",
						Name:               identifiableName,
						Controller:         &falseVal,
						BlockOwnerDeletion: &trueVal,
					}}),
				),
			},
			config:      ConfigMapFromTestFile(t, "config-gcp-auth", "default-auth-config"),
			wantErrCode: codes.NotFound,
		}, {
			name: "default serviceAccountName, no need to remove k8s service account",
			objects: []runtime.Object{
				NewServiceAccount(kServiceAccountName, testNS,
					WithServiceAccountAnnotation(gServiceAccountName),
					WithServiceAccountOwnerReferences([]metav1.OwnerReference{{
						APIVersion:         "events.cloud.google.com/v1beta1",
						Kind:               "CloudPubSubSource",
						UID:                "test-pubsub-uid1",
						Name:               identifiableName,
						Controller:         &falseVal,
						BlockOwnerDeletion: &trueVal,
					}, {
						APIVersion:         "events.cloud.google.com/v1beta1",
						Kind:               "CloudPubSubSource",
						UID:                "test-pubsub-uid2",
						Name:               identifiableName + "new",
						Controller:         &falseVal,
						BlockOwnerDeletion: &trueVal,
					}}),
				),
			},
			config: ConfigMapFromTestFile(t, "config-gcp-auth", "default-auth-config"),
		}}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cs := fakeKubeClient.NewSimpleClientset(tc.objects...)
			iamClient := gclient.NewTestClient()
			m, err := iam.NewIAMPolicyManager(ctx, iamClient)
			if err != nil {
				t.Fatal(err)
			}
			identity := &Identity{
				kubeClient:    cs,
				policyManager: m,
				gcpAuthStore:  NewGCPAuthTestStore(t, tc.config),
			}
			identifiable := v1.NewCloudPubSubSource(identifiableName, testNS,
				v1.WithCloudPubSubSourceSetDefaults,
			)
			identifiable.Spec.ServiceAccountName = kServiceAccountName
			identifiable.SetAnnotations(map[string]string{
				duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
			})

			arl := pkgtesting.ActionRecorderList{cs}
			err = identity.DeleteWorkloadIdentity(ctx, projectID, identifiable)

			var statusErr interface{ GRPCStatus() *status.Status }
			if errors.As(err, &statusErr) {
				if code := statusErr.GRPCStatus().Code(); code != tc.wantErrCode {
					t.Fatalf("error code: want %v, got %v", tc.wantErrCode, code)
				}
			} else {
				if tc.wantErrCode != codes.OK {
					t.Fatal(err)
				}
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
		})
	}
}

func TestOwnerReferenceExists(t *testing.T) {
	t.Parallel()
	source := &v1beta1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduler-name",
			Namespace: "scheduler-namespace",
			UID:       "scheduler-uid",
		},
		Spec: v1beta1.CloudSchedulerSourceSpec{
			PubSubSpec: duckv1beta1.PubSubSpec{
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
