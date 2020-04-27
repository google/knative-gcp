/*
Copyright 2020 Google LLC

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trigger

import (
	"context"
	"errors"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/pubsub/v1alpha1/trigger"
	gtrigger "github.com/google/knative-gcp/pkg/gclient/trigger/testing"

	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	triggerName = "my-test-trigger"
	triggerUID  = "test-trigger-uid"
	sourceType  = "AUDIT"

	testNS        = "testnamespace"
	testProject   = "test-project-id"
	testTriggerID = "trigger-" + triggerUID
	generation    = 1

	// Message for when the topic and pullsubscription with the above variables are not ready.
	failedToReconcileTopicMsg                  = `Topic has not yet been reconciled`
	failedToReconcilepullSubscriptionMsg       = `PullSubscription has not yet been reconciled`
	failedToReconcileTriggerMsg                = `Failed to reconcile Trigger Event flow trigger`
	failedToReconcilePubSubMsg                 = `Failed to reconcile Trigger PubSub`
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
	failedToDeleteTriggerMsg                   = `Failed to delete Trigger trigger`
)

var (
	trueVal         = true
	falseVal        = false
	gServiceAccount = "test123@test123.iam.gserviceaccount.com"
)

// Returns an ownerref for the test Trigger object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "pubsub.cloud.google.com/v1alpha1",
		Kind:               "Trigger",
		Name:               "my-test-trigger",
		UID:                triggerUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}

func patchFinalizers(namespace, name string, add bool) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	var fname string
	if add {
		fname = fmt.Sprintf("%q", resourceGroup)
	}

	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func TestAllCases(t *testing.T) {

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "create client fails",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				}),
			),
		},
		Key: testNS + "/" + triggerName,
		OtherTestData: map[string]interface{}{
			"trigger": gtrigger.TestClientData{
				CreateClientErr: errors.New("create-client-induced-error"),
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: create-client-induced-error"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerProjectID(testProject),
				WithPubSubTriggerStatusObservedGeneration(generation),
				WithInitPubSubTriggerConditions,
				WithPubSubTriggerTriggerNotReady(reconciledTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: create-client-induced-error"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				})),
		}},
	}, {
		Name: "verify trigger exists fails",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				}),
			),
		},
		Key: testNS + "/" + triggerName,
		OtherTestData: map[string]interface{}{
			"trigger": gtrigger.TestClientData{
				TriggerData: gtrigger.TestTriggerData{
					ExistsErr: errors.New("trigger-exists-induced-error"),
				},
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: trigger-exists-induced-error"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerProjectID(testProject),
				WithPubSubTriggerStatusObservedGeneration(generation),
				WithInitPubSubTriggerConditions,
				WithPubSubTriggerTriggerNotReady(reconciledTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: trigger-exists-induced-error"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				})),
		}},
	}, {
		Name: "create trigger fails",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				}),
			),
		},
		Key: testNS + "/" + triggerName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: create-trigger-induced-error"),
		},
		OtherTestData: map[string]interface{}{
			"trigger": gtrigger.TestClientData{
				CreateTriggerErr: errors.New("create-trigger-induced-error"),
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerProjectID(testProject),
				WithPubSubTriggerStatusObservedGeneration(generation),
				WithInitPubSubTriggerConditions,
				WithPubSubTriggerTriggerNotReady(reconciledTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: create-trigger-induced-error"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				})),
		}},
	}, {
		Name: "successfully created trigger",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				}),
			),
		},
		Key: testNS + "/" + triggerName,
		OtherTestData: map[string]interface{}{
			"trigger": gtrigger.TestClientData{
				TriggerData: gtrigger.TestTriggerData{
					Exists: true,
				},
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeNormal, reconciledSuccess, `Trigger reconciled: "%s/%s"`, testNS, triggerName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerStatusObservedGeneration(generation),
				WithInitPubSubTriggerConditions,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerProjectID(testProject),
				WithPubSubTriggerTriggerReady(testTriggerID),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				}),
			),
		}},
	}, {
		Name: "successfully deleted trigger",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerSpec(v1alpha1.TriggerSpec{
					Project: testProject,
					Trigger: testTriggerID,
					/*IdentitySpec: duckv1alpha1.IdentitySpec{
						GoogleServiceAccount: gServiceAccount,
					},*/
					SourceType: sourceType,
					Filters: map[string]string{
						"ServiceName":  "foo",
						"MethodName":   "bar",
						"ResourceName": "baz",
					},
				}),
				WithPubSubTriggerDeletionTimestamp(),
			),
		},
		Key: testNS + "/" + triggerName,
		OtherTestData: map[string]interface{}{
			"trigger": gtrigger.TestClientData{
				TriggerData: gtrigger.TestTriggerData{
					Exists: true,
				},
			},
		},
		WantEvents:        nil,
		WantStatusUpdates: nil,
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase:           pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			Identity:             identity.NewIdentity(ctx, iam.DefaultIAMPolicyManager()),
			triggerLister:        listers.Getv1alpha1PubSubTriggerLister(),
			serviceAccountLister: listers.GetServiceAccountLister(),
			createClientFn:       gtrigger.TestClientCreator(testData["trigger"]),
		}
		return trigger.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.Getv1alpha1PubSubTriggerLister(), r.Recorder, r)
	}))

}
