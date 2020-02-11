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
	apierrs "k8s.io/apimachinery/pkg/api/errors"
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

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	storagev1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	gstorage "github.com/google/knative-gcp/pkg/gclient/storage/testing"
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
	failedToDeleteNotificationMsg        = `Failed to delete CloudStorageSource notification`
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
	attempts := 0
	storageSinkURL := sinkURL(t, sinkURI)

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
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
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
			Eventf(corev1.EventTypeNormal, "Updated", "Updated CloudStorageSource %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q has not yet been reconciled", storageName),
		},
	}, {
		Name: "topic exists, topic not yet been reconciled",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicTopicID(testTopicID),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q has not yet been reconciled", storageName),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
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
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" did not expose projectid`),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose projectid", storageName),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
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
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" did not expose topicid`),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose topicid", storageName),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
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
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" mismatch: expected "storage-test-storage-uid" got "garbaaaaage"`),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Topic %q mismatch: expected "storage-test-storage-uid" got "garbaaaaage"`, storageName),
		},
	}, {
		Name: "topic exists and the status of topic is false",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicFailed(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicFailed("PublisherStatus", "Publisher has no Ready type status"),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "the status of Topic %q is False", storageName),
		},
	}, {
		Name: "topic exists and the status of topic is unknown",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicUnknown(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicUnknown("", ""),
				WithCloudStorageSourceFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "the status of Topic %q is Unknown", storageName),
		},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithCloudStorageSourceFinalizers(finalizerName),
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
			Object: NewCloudStorageSource(storageName, testNS,
				WithCloudStorageSourceObjectMetaGeneration(generation),
				WithCloudStorageSourceBucket(bucket),
				WithCloudStorageSourceSink(sinkGVK, sinkName),
				WithInitCloudStorageSourceConditions,
				WithCloudStorageSourceTopicReady(testTopicID),
				WithCloudStorageSourceProjectID(testProject),
				WithCloudStorageSourceFinalizers(finalizerName),
				WithCloudStorageSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
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
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": storageName,
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q has not yet been reconciled", storageName),
		},
	},
		{
			Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceFinalizers(finalizerName),
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
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q has not yet been reconciled", storageName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceFinalizers(finalizerName),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS, WithPullSubscriptionFailed()),
			},
			Key:     testNS + "/" + storageName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "the status of PullSubscription %q is False", storageName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceFinalizers(finalizerName),
				),
				NewTopic(storageName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(storageName, testNS, WithPullSubscriptionUnknown()),
			},
			Key:     testNS + "/" + storageName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionUnknown("", ""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "the status of PullSubscription %q is Unknown", storageName),
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
					WithCloudStorageSourceFinalizers(finalizerName),
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
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady("NotificationReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "create-client-induced-error")),
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
					WithCloudStorageSourceFinalizers(finalizerName),
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
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "bucket-notifications-induced-error"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady("NotificationReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-notifications-induced-error")),
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
					WithCloudStorageSourceFinalizers(finalizerName),
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
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "bucket-add-notification-induced-error"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady("NotificationReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-add-notification-induced-error")),
				),
			}},
		}, {
			Name: "successfully created notification, with retry",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
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
			WithReactors: []clientgotesting.ReactionFunc{
				func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					if attempts != 0 || !action.Matches("update", "cloudstoragesources") {
						return false, nil, nil
					}
					attempts++
					return true, nil, apierrs.NewConflict(v1alpha1.Resource("foo"), "bar", errors.New("foo"))
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ReadinessChanged", "CloudStorageSource %q became ready", storageName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated CloudStorageSource %q", storageName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithCloudStorageSourceProjectID(testProject),
					WithCloudStorageSourcePullSubscriptionReady(),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationReady(notificationId)),
			}, {
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
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
					WithCloudStorageSourceFinalizers(finalizerName),
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
				Eventf(corev1.EventTypeWarning, "InternalError", "delete-notification-induced-error"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady("NotificationDeleteFailed", fmt.Sprintf("%s: %s", failedToDeleteNotificationMsg, "delete-notification-induced-error")),
					WithCloudStorageSourceNotificationID(notificationId),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithDeletionTimestamp()),
			}},
		}, {
			Name: "delete fails with Unknown grpc error",
			Objects: []runtime.Object{
				NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
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
				Eventf(corev1.EventTypeWarning, "InternalError", "rpc error: code = %s desc = %s", codes.Unknown, "delete-notification-induced-error"),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceSinkURI(storageSinkURL),
					WithCloudStorageSourceNotificationNotReady("NotificationDeleteFailed", fmt.Sprintf("%s: rpc error: code = %s desc = %s", failedToDeleteNotificationMsg, codes.Unknown, "delete-notification-induced-error")),
					WithCloudStorageSourceNotificationID(notificationId),
					WithCloudStorageSourceTopicReady(testTopicID),
					WithDeletionTimestamp()),
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
					WithCloudStorageSourceFinalizers(finalizerName),
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
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated CloudStorageSource %q finalizers", storageName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated CloudStorageSource %q", storageName),
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
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudStorageSource(storageName, testNS,
					WithCloudStorageSourceProject(testProject),
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceStatusObservedGeneration(generation),
					WithCloudStorageSourceBucket(bucket),
					WithCloudStorageSourceSink(sinkGVK, sinkName),
					WithCloudStorageSourceEventTypes([]string{storagev1alpha1.CloudStorageSourceFinalize}),
					WithCloudStorageSourceFinalizers(finalizerName),
					WithInitCloudStorageSourceConditions,
					WithCloudStorageSourceObjectMetaGeneration(generation),
					WithCloudStorageSourceNotificationNotReady("NotificationDeleted", fmt.Sprintf("Successfully deleted CloudStorageSource notification: %s", notificationId)),
					WithCloudStorageSourceTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", storageName)),
					WithCloudStorageSourcePullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", storageName)),
					WithDeletionTimestamp()),
			}},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		return &Reconciler{
			PubSubBase:     pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			storageLister:  listers.GetCloudStorageSourceLister(),
			createClientFn: gstorage.TestClientCreator(testData["storage"]),
		}
	}))

}
