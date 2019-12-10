/*
Copyright 2019 Google LLC

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

package storage

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	storagev1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	storageName    = "my-test-storage"
	storageUID     = "test-storage-uid"
	bucket         = "my-test-bucket"
	sinkName       = "sink"
	notificationId = "135"

	testNS       = "testnamespace"
	testImage    = "notification-ops-image"
	testProject  = "test-project-id"
	testTopicID  = "storage-" + storageUID
	testTopicURI = "http://" + storageName + "-topic." + testNS + ".svc.cluster.local"
	generation   = 1
)

var (
	trueVal = true

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = "http://" + sinkDNS + "/"

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

	// Message for when the topic and pullsubscription with the above variables are not ready.
	topicNotReadyMsg            = `Topic "my-test-storage" not ready`
	pullSubscriptionNotReadyMsg = `PullSubscription "my-test-storage" not ready`
)

func init() {
	// Add types to scheme
	_ = storagev1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test Storage object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "Storage",
		Name:               "my-test-storage",
		UID:                storageUID,
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
		fname = fmt.Sprintf("%q", finalizerName)
	}
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func newSink() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testing.cloud.google.com/v1alpha1",
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

// turn string into URL or terminate with t.Fatalf
func sinkURL(t *testing.T, url string) *apis.URL {
	u, err := apis.ParseURL(url)
	if err != nil {
		t.Fatalf("Failed to parse url %q", url)
	}
	return u
}

func TestAllCases(t *testing.T) {
	//storageSinkURL := sinkURL(t, sinkURI)

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "topic created, not ready",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(storageName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Storage %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q not ready", storageName),
		},
	}, {
		Name: "topic exists, topic not ready",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicTopicID(testTopicID),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q not ready", storageName),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", `Topic "my-test-storage" did not expose projectid`),
				WithStorageFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose projectid", storageName),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", `Topic "my-test-storage" did not expose topicid`),
				WithStorageFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose topicid", storageName),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", `Topic "my-test-storage" mismatch: expected "storage-test-storage-uid" got "garbaaaaage"`),
				WithStorageFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Topic %q mismatch: expected "storage-test-storage-uid" got "garbaaaaage"`, storageName),
		},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicReady(testTopicID),
				WithStorageProjectID(testProject),
				WithStorageFinalizers(finalizerName),
				WithStoragePullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(storageName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic:  testTopicID,
					Secret: &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q not ready", storageName),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but is not ready",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(storageName, testNS),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageObjectMetaGeneration(generation),
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
				WithInitStorageConditions,
				WithStorageTopicReady(testTopicID),
				WithStorageProjectID(testProject),
				WithStoragePullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q not ready", storageName),
		},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job created",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageEventTypes([]string{"finalize"}),
		//			WithStorageFinalizers(finalizerName),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageEventTypes([]string{"finalize"}),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", jobNotCompletedMsg),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//	WantCreates: []runtime.Object{
		//		newJob(NewStorage(storageName, testNS), ops.ActionCreate),
		//	},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job not finished",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithStorageObjectMetaGeneration(generation),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJob(NewStorage(storageName, testNS), ops.ActionCreate),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", jobNotCompletedMsg),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job failed",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithStorageObjectMetaGeneration(generation),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJobFinished(NewStorage(storageName, testNS), ops.ActionCreate, false),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", jobFailedMsg),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job finished, no pods found",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJobFinished(NewStorage(storageName, testNS), ops.ActionCreate, true),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", jobPodNotFoundMsg),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job finished, no termination msg on pod",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJobFinished(NewStorage(storageName, testNS), ops.ActionCreate, true),
		//		newPod(""),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", `Failed to create Storage notification: did not find termination message for pod "test-pod"`),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job finished, invalid termination msg on pod",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJobFinished(NewStorage(storageName, testNS), ops.ActionCreate, true),
		//		newPod("invalid msg"),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", `Failed to create Storage notification: failed to unmarshal terminationmessage: "invalid msg" : "invalid character 'i' looking for beginning of value"`),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job finished, notification creation failed",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJobFinished(NewStorage(storageName, testNS), ops.ActionCreate, true),
		//		newPod(string(failureMsg)),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageTopicReady(testTopicID),
		//			WithStorageProjectID(testProject),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationNotReady("NotificationNotReady", "Failed to create Storage notification: operation failed: test induced failure"),
		//			WithStorageSinkURI(storageSinkURL),
		//		),
		//	}},
		//}, {
		//	Name: "topic and pullsubscription exist and ready, notification job finished, notification created successfully",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//		newJobFinished(NewStorage(storageName, testNS), ops.ActionCreate, true),
		//		newPod(string(successMsg)),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: false,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageFinalizers(finalizerName),
		//			WithInitStorageConditions,
		//			WithStorageTopicReady(testTopicID),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationReady(),
		//			WithStorageSinkURI(storageSinkURL),
		//			WithStorageNotificationID(notificationId),
		//			WithStorageProjectID(testProject),
		//		),
		//	}},
		//}, {
		//	Name: "deletion timestamp set, all ready, delete job created",
		//	Objects: []runtime.Object{
		//		NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageProjectID(testProject),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageEventTypes([]string{"finalize"}),
		//			WithStorageTopicReady(testTopicID),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationReady(),
		//			WithStorageFinalizers(finalizerName),
		//			WithStorageNotificationID(notificationId),
		//			WithDeletionTimestamp(),
		//		),
		//		NewTopic(storageName, testNS,
		//			WithTopicReady(testTopicID),
		//			WithTopicAddress(testTopicURI),
		//			WithTopicProjectID(testProject),
		//		),
		//		NewPullSubscriptionWithNoDefaults(storageName, testNS,
		//			WithPullSubscriptionReady(sinkURI),
		//		),
		//		newSink(),
		//	},
		//	Key:     testNS + "/" + storageName,
		//	WantErr: true,
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewStorage(storageName, testNS,
		//			WithStorageObjectMetaGeneration(generation),
		//			WithStorageStatusObservedGeneration(generation),
		//			WithStorageBucket(bucket),
		//			WithStorageProjectID(testProject),
		//			WithStorageSink(sinkGVK, sinkName),
		//			WithStorageEventTypes([]string{"finalize"}),
		//			WithStorageTopicReady(testTopicID),
		//			WithStoragePullSubscriptionReady(),
		//			WithStorageNotificationReady(),
		//			WithStorageFinalizers(finalizerName),
		//			WithStorageNotificationID(notificationId),
		//			WithDeletionTimestamp(),
		//		),
		//	}},
		//	WantCreates: []runtime.Object{
		//		newJob(NewStorage(storageName, testNS), ops.ActionDelete),
		//	},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, _ map[string]interface{}) controller.Reconciler {
		return &Reconciler{
			PubSubBase:    pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			storageLister: listers.GetStorageLister(),
		}
	}))

}

//func newJob(owner kmeta.OwnerRefable, action string) runtime.Object {
//	if action == "create" {
//		j, _ := operations.NewNotificationOps(operations.NotificationArgs{
//			UID:        storageUID,
//			Image:      testImage,
//			Action:     ops.ActionCreate,
//			ProjectID:  testProject,
//			Bucket:     bucket,
//			TopicID:    testTopicID,
//			EventTypes: []string{"finalize"},
//			Secret:     secret,
//			Owner:      owner,
//		})
//		return j
//	}
//	j, _ := operations.NewNotificationOps(operations.NotificationArgs{
//		UID:            storageUID,
//		Image:          testImage,
//		Action:         ops.ActionDelete,
//		ProjectID:      testProject,
//		Bucket:         bucket,
//		NotificationId: notificationId,
//		Secret:         secret,
//		Owner:          owner,
//	})
//	return j
//}
//
//func newJobFinished(owner kmeta.OwnerRefable, action string, success bool) runtime.Object {
//	var job *batchv1.Job
//	if action == "create" {
//		job, _ = operations.NewNotificationOps(operations.NotificationArgs{
//			UID:        storageUID,
//			Image:      testImage,
//			Action:     ops.ActionCreate,
//			ProjectID:  testProject,
//			Bucket:     bucket,
//			TopicID:    testTopicID,
//			EventTypes: []string{"finalize"},
//			Secret:     secret,
//			Owner:      owner,
//		})
//	} else {
//		job, _ = operations.NewNotificationOps(operations.NotificationArgs{
//			UID:            storageUID,
//			Image:          testImage,
//			Action:         ops.ActionDelete,
//			ProjectID:      testProject,
//			Bucket:         bucket,
//			NotificationId: notificationId,
//			Secret:         secret,
//			Owner:          owner,
//		})
//	}
//
//	if success {
//		job.Status.Active = 0
//		job.Status.Succeeded = 1
//		job.Status.Conditions = []batchv1.JobCondition{{
//			Type:   batchv1.JobComplete,
//			Status: corev1.ConditionTrue,
//		}, {
//			Type:   batchv1.JobFailed,
//			Status: corev1.ConditionFalse,
//		}}
//	} else {
//		job.Status.Active = 0
//		job.Status.Succeeded = 0
//		job.Status.Conditions = []batchv1.JobCondition{{
//			Type:   batchv1.JobComplete,
//			Status: corev1.ConditionFalse,
//		}, {
//			Type:   batchv1.JobFailed,
//			Status: corev1.ConditionTrue,
//		}}
//	}
//
//	return job
//}

func newPod(msg string) runtime.Object {
	labels := map[string]string{
		"resource-uid": storageUID,
		"action":       "create",
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: testNS,
			Labels:    labels,
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: msg,
						},
					},
				},
			},
		},
	}
}
