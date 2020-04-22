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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	"github.com/google/knative-gcp/pkg/client/injection/reconciler/pubsub/v1beta1/trigger"

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

	sinkName  = "sink"
	triggerId = "135"
	testNS    = "testnamespace"
	//testImage      = "notification-ops-image"
	testProject  = "test-project-id"
	testTopicID  = "trigger-" + triggerUID
	testTopicURI = "http://" + triggerName + "-topic." + testNS + ".svc.cluster.local"
	generation   = 1

	// Message for when the topic and pullsubscription with the above variables are not ready.
	failedToReconcileTopicMsg                  = `Topic has not yet been reconciled`
	failedToReconcilepullSubscriptionMsg       = `PullSubscription has not yet been reconciled`
	failedToReconcileTriggerMsg                = `Failed to reconcile Trigger Event flow trigger`
	failedToReconcilePubSubMsg                 = `Failed to reconcile Trigger PubSub`
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
	failedToDeleteTriggerMsg                   = `Failed to delete Trigger trigger`
)

var (
	trueVal  = true
	falseVal = false

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = apis.HTTP(sinkDNS)

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "Sink",
	}

	secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "google-cloud-key",
		},
		Key: "key.json",
	}

	gServiceAccount = "test123@test123.iam.gserviceaccount.com"
)

// Returns an ownerref for the test Trigger object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "pubsub.cloud.google.com/v1beta1",
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

func newSink() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testing.cloud.google.com/v1beta1",
			"kind":       "Sink",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      sinkName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": sinkDNS,
				},
			},
		},
	}
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
		Name: "delete fails with getting k8s service account error",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerProject(testProject),
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerSourceType(sourceType),
				WithPubSubTriggerFilter("ServiceName", "foo"),
				WithPubSubTriggerFilter("MethodName", "bar"),
				WithPubSubTriggerFilter("ResourceName", "baz"),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerGCPServiceAccount(gServiceAccount),
				WithPubSubTriggerDeletionTimestamp(),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "WorkloadIdentityDeleteFailed", `Failed to delete Trigger workload identity: getting k8s service account failed with: serviceaccounts "test123" not found`),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerProject(testProject),
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerSourceType(sourceType),
				WithPubSubTriggerFilter("ServiceName", "foo"),
				WithPubSubTriggerFilter("MethodName", "bar"),
				WithPubSubTriggerFilter("ResourceName", "baz"),
				WithPubSubTriggerGCPServiceAccount(gServiceAccount),
				WithPubSubTriggerServiceAccountName("test123"),
				WithPubSubTriggerWorkloadIdentityFailed("WorkloadIdentityDeleteFailed", `serviceaccounts "test123" not found`),
				WithPubSubTriggerDeletionTimestamp(),
			),
		}},
	}, {
		Name: "successfully deleted trigger",
		Objects: []runtime.Object{
			NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerProject(testProject),
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerSourceType(sourceType),
				WithPubSubTriggerFilter("ServiceName", "foo"),
				WithPubSubTriggerFilter("MethodName", "bar"),
				WithPubSubTriggerFilter("ResourceName", "baz"),
				WithPubSubTriggerDeletionTimestamp(),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(triggerName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
			newSink(),
		},
		Key:           testNS + "/" + triggerName,
		OtherTestData: map[string]interface{}{
			// TODO(nlopezgi): add TestClientData for reconciler client once added
			/*"google": ggoogle.TestClientData{
				BucketData: ggoogle.TestBucketData{
					Notifications: map[string]*google.Notification{
						triggerId: {
							ID: triggerId,
						},
					},
				},
			},*/
		},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}},
				Name: triggerName,
			},
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}},
				Name: triggerName,
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubTrigger(triggerName, testNS,
				WithPubSubTriggerProject(testProject),
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerSourceType(sourceType),
				WithPubSubTriggerFilter("ServiceName", "foo"),
				WithPubSubTriggerFilter("MethodName", "bar"),
				WithPubSubTriggerFilter("ResourceName", "baz"),
				WithPubSubTriggerObjectMetaGeneration(generation),
				WithPubSubTriggerDeletionTimestamp(),
			),
		}},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase:           pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			Identity:             identity.NewIdentity(ctx, iam.DefaultIAMPolicyManager()),
			triggerLister:        listers.GetPubSubTriggerLister(),
			serviceAccountLister: listers.GetServiceAccountLister(),
		}
		return trigger.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetPubSubTriggerLister(), r.Recorder, r)
	}))

}
