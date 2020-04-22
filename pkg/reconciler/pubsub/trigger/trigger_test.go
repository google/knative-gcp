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

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/pubsub/v1beta1/trigger"

	"github.com/google/knative-gcp/pkg/reconciler/identity"
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
	googleSinkURL := sinkURI

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "topic created, not yet been reconciled",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(triggerName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": triggerName,
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: Topic %q has not yet been reconciled", failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists, topic not yet been reconciled",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicTopicID(testTopicID),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of Topic %q is Unknown", failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicFailed("TopicNotReady", `Topic "my-test-trigger" did not expose projectid`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: Topic %q did not expose projectid", failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicFailed("TopicNotReady", `Topic "my-test-trigger" did not expose topicid`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: Topic %q did not expose topicid", failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicFailed("TopicNotReady", `Topic "my-test-trigger" mismatch: expected "trigger-test-trigger-uid" got "garbaaaaage"`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf(`%s: Topic %q mismatch: expected "trigger-test-trigger-uid" got "garbaaaaage"`, failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists and the status of topic is false",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicFailed(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicFailed("PublisherStatus", "Publisher has no Ready type status"),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of Topic %q is False", failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists and the status of topic is unknown",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicUnknown(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicUnknown("", ""),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of Topic %q is Unknown", failedToReconcilePubSubMsg, triggerName)),
		},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicReady(testTopicID),
				WithTriggerProjectID(testProject),
				WithTriggerPullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(triggerName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret: &secret,
					},
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": triggerName,
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: PullSubscription %q has not yet been reconciled", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, triggerName)),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(triggerName, testNS),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicReady(testTopicID),
				WithTriggerProjectID(testProject),
				WithTriggerPullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: PullSubscription %q has not yet been reconciled", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, triggerName)),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(triggerName, testNS, WithPullSubscriptionFailed()),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicReady(testTopicID),
				WithTriggerProjectID(testProject),
				WithTriggerPullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: the status of PullSubscription %q is False", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, triggerName)),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
			),
			NewTopic(triggerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(triggerName, testNS, WithPullSubscriptionUnknown()),
		},
		Key: testNS + "/" + triggerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerStatusObservedGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithInitTriggerConditions,
				WithTriggerTopicReady(testTopicID),
				WithTriggerProjectID(testProject),
				WithTriggerPullSubscriptionUnknown("", ""),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, triggerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", triggerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: the status of PullSubscription %q is Unknown", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, triggerName)),
		},
	}, {
		Name: "delete fails with getting k8s service account error",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerProject(testProject),
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithTriggerSinkURI(googleSinkURL),
				WithTriggerServiceAccountName("test123"),
				WithTriggerGCPServiceAccount(gServiceAccount),
				WithTriggerDeletionTimestamp(),
			),
			newSink(),
		},
		Key: testNS + "/" + triggerName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "WorkloadIdentityDeleteFailed", `Failed to delete Trigger workload identity: getting k8s service account failed with: serviceaccounts "test123" not found`),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTrigger(triggerName, testNS,
				WithTriggerProject(testProject),
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithTriggerSinkURI(googleSinkURL),
				WithTriggerGCPServiceAccount(gServiceAccount),
				WithTriggerServiceAccountName("test123"),
				WithTriggerWorkloadIdentityFailed("WorkloadIdentityDeleteFailed", `serviceaccounts "test123" not found`),
				WithTriggerDeletionTimestamp(),
			),
		}},
	}, {
		Name: "successfully deleted google",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS,
				WithTriggerProject(testProject),
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithTriggerSinkURI(googleSinkURL),
				WithTriggerTopicReady(testTopicID),
				WithTriggerDeletionTimestamp(),
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
			Object: NewTrigger(triggerName, testNS,
				WithTriggerProject(testProject),
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerSourceType(sourceType),
				WithTriggerFilter("ServiceName", "foo"),
				WithTriggerFilter("MethodName", "bar"),
				WithTriggerFilter("ResourceName", "baz"),
				WithTriggerSink(sinkGVK, sinkName),
				WithTriggerObjectMetaGeneration(generation),
				WithTriggerTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", triggerName)),
				WithTriggerPullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", triggerName)),
				WithTriggerDeletionTimestamp(),
			),
		}},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase:           pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			Identity:             identity.NewIdentity(ctx),
			triggerLister:        listers.GetTriggerLister(),
			serviceAccountLister: listers.GetServiceAccountLister(),
		}
		return trigger.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetTriggerLister(), r.Recorder, r)
	}))

}
