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
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	storagev1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudstoragesource"
	gstorage "github.com/google/knative-gcp/pkg/gclient/storage/testing"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
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
	testNS         = "testnamespace"
	testImage      = "notification-ops-image"
	testProject    = "test-project-id"
	testTopicID    = "storage-" + storageUID
	testTopicURI   = "http://" + storageName + "-topic." + testNS + ".svc.cluster.local"
	generation     = 1

	// Message for when the topic and pullsubscription with the above variables are not ready.
	failedToReconcileTopicMsg            = `Topic has not yet been reconciled`
	failedToReconcilepullSubscriptionMsg = `PullSubscription has not yet been reconciled`
	failedToReconcileNotificationMsg     = `Failed to reconcile CloudStorageSource notification`
	failedToReconcilePubSubMsg           = `Failed to reconcile CloudStorageSource PubSub`
	failedToDeleteNotificationMsg        = `Failed to delete CloudStorageSource notification`
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

func init() {
	// Add types to scheme
	_ = storagev1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test CloudStorageSource object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "CloudStorageSource",
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
		fname = fmt.Sprintf("%q", resourceGroup)
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

// TODO add a unit test for successfully creating a k8s service account, after issue https://github.com/google/knative-gcp/issues/657 gets solved.
func TestAllCases(t *testing.T) {
	storageSinkURL := sinkURI

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
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(storageName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": storageName,
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: Topic %q has not yet been reconciled", failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists, topic not yet been reconciled",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicTopicID(testTopicID),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of Topic %q is Unknown", failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" did not expose projectid`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: Topic %q did not expose projectid", failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" did not expose topicid`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: Topic %q did not expose topicid", failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" mismatch: expected "storage-test-storage-uid" got "garbaaaaage"`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf(`%s: Topic %q mismatch: expected "storage-test-storage-uid" got "garbaaaaage"`, failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists and the status of topic is false",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicFailed(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("PublisherStatus", "Publisher has no Ready type status"),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of Topic %q is False", failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists and the status of topic is unknown",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicUnknown(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicUnknown("", ""),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of Topic %q is Unknown", failedToReconcilePubSubMsg, storageName)),
		},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceStatusObservedGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicReady(testTopicID),
				WithCloudStorageSourceProjectID(testProject),
				WithCloudStorageSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(storageName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret: &secret,
					},
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": storageName,
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToReconcilePubSubMsg, storageName)),
		},
	},
		{
			Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToReconcilePubSubMsg, storageName)),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS, WithPullSubscriptionFailed()),
			},
			Key: testNS + "/" + storageName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of PullSubscription %q is False", failedToReconcilePubSubMsg, storageName)),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS, WithPullSubscriptionUnknown()),
			},
			Key: testNS + "/" + storageName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionUnknown("", ""),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: the status of PullSubscription %q is Unknown", failedToReconcilePubSubMsg, storageName)),
			},
		},
		{
			Name: "client create fails",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					CreateClientErr: errors.New("create-client-induced-error"),
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "create-client-induced-error")),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "create-client-induced-error")),
				),
			}},
		}, {
			Name: "bucket notifications fails",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						NotificationsErr: errors.New("bucket-notifications-induced-error"),
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-notifications-induced-error")),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-notifications-induced-error")),
				),
			}},
		}, {
			Name: "bucket add notification fails",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						AddNotificationErr: errors.New("bucket-add-notification-induced-error"),
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-add-notification-induced-error")),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-add-notification-induced-error")),
				),
			}},
		}, {
			Name: "successfully created notification",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						AddNotificationID: notificationId,
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `CloudStorageSource reconciled: "%s/%s"`, testNS, storageName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationReady(notificationId)),
			}},
		}, {
			Name: "delete fails with non grpc error",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationReady(notificationId),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithDeletionTimestamp(),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						Notifications: map[string]*storage.Notification{
							notificationId: {
								ID: notificationId,
							},
						},
						DeleteErr: errors.New("delete-notification-induced-error"),
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, deleteNotificationFailed, "Failed to delete CloudStorageSource notification: delete-notification-induced-error"),
			},
			WantStatusUpdates: nil,
		}, {
			Name: "delete fails with Unknown grpc error",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationReady(notificationId),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithDeletionTimestamp(),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						Notifications: map[string]*storage.Notification{
							notificationId: {
								ID: notificationId,
							},
						},
						DeleteErr: gstatus.Error(codes.Unknown, "delete-notification-induced-error"),
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, deleteNotificationFailed, "Failed to delete CloudStorageSource notification: rpc error: code = %s desc = %s", codes.Unknown, "delete-notification-induced-error"),
			},
			WantStatusUpdates: nil,
		}, {
			Name: "delete fails with getting k8s service account error",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceServiceAccountName("test123"),
					WithCloudStorageSourceGCPServiceAccount(gServiceAccount),
					WithDeletionTimestamp(),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "WorkloadIdentityDeleteFailed", `Failed to delete CloudStorageSource workload identity: getting k8s service account failed with: serviceaccounts "test123" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceGCPServiceAccount(gServiceAccount),
					WithCloudStorageSourceServiceAccountName("test123"),
					WithCloudStorageSourceWorkloadIdentityFailed("WorkloadIdentityDeleteFailed", `serviceaccounts "test123" not found`),
					WithDeletionTimestamp(),
				),
			}},
		}, {
			Name: "successfully deleted storage",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithDeletionTimestamp(),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						Notifications: map[string]*storage.Notification{
							notificationId: {
								ID: notificationId,
							},
						},
					},
				},
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}},
					Name: storageName,
				},
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}},
					Name: storageName,
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", storageName)),
					WithCloudStorageSourcePullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", storageName)),
					WithDeletionTimestamp()),
			}},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase:           pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			Identity:             identity.NewIdentity(ctx),
			storageLister:        listers.GetCloudStorageSourceLister(),
			createClientFn:       gstorage.TestClientCreator(testData["storage"]),
			serviceAccountLister: listers.GetServiceAccountLister(),
		}
		return cloudstoragesource.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetCloudStorageSourceLister(), r.Recorder, r)
	}))

}
