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

	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"

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
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"

	"github.com/google/knative-gcp/pkg/apis/duck"
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	storagev1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	. "github.com/google/knative-gcp/pkg/apis/intevents"
	inteventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudstoragesource"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	gstorage "github.com/google/knative-gcp/pkg/gclient/storage/testing"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
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
	testTopicURI   = "http://" + storageName + "-topic." + testNS + ".svc.cluster.local"
	generation     = 1

	// Message for when the topic and pullsubscription with the above variables are not ready.
	failedToReconcileTopicMsg                  = `Topic has not yet been reconciled`
	failedToReconcilepullSubscriptionMsg       = `PullSubscription has not yet been reconciled`
	failedToReconcileNotificationMsg           = `Failed to reconcile CloudStorageSource notification`
	failedToReconcilePubSubMsg                 = `Failed to reconcile CloudStorageSource PubSub`
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
	failedToDeleteNotificationMsg              = `Failed to delete CloudStorageSource notification`
)

var (
	trueVal  = true
	falseVal = false

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = apis.HTTP(sinkDNS)

	testTopicID = fmt.Sprintf("cre-src_%s_%s_%s", testNS, storageName, storageUID)

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1",
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
	_ = storagev1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test CloudStorageSource object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1",
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
			"apiVersion": "testing.cloud.google.com/v1",
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

func newSinkDestination() duckv1.Destination {
	return duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "testing.cloud.google.com/v1",
			Kind:       "Sink",
			Name:       sinkName,
			Namespace:  testNS,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				reconcilertestingv1.WithCloudStorageSourceTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
		}},
		WantCreates: []runtime.Object{
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
					SourceLabelKey:    storageName,
				}),
				reconcilertestingv1.WithTopicAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicSetDefaults,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicUnknown,
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" did not expose projectid`),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(""),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicFailed("TopicNotReady", `Topic "my-test-storage" did not expose topicid`),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady("garbaaaaage"),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicFailed("TopicNotReady", fmt.Sprintf(`Topic "my-test-storage" mismatch: expected %q got "garbaaaaage"`, testTopicID)),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf(`%s: Topic %q mismatch: expected "%s" got "garbaaaaage"`, failedToReconcilePubSubMsg, storageName, testTopicID)),
		},
	}, {
		Name: "topic exists and the status of topic is false",
		Objects: []runtime.Object{
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicFailed,
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicFailed("TopicFailed", "test message"),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicUnknown,
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicUnknown("", ""),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
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
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
				reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
				reconcilertestingv1.WithCloudStorageSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				reconcilertestingv1.WithCloudStorageSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
			),
		}},
		WantCreates: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(storageName, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
					},
					AdapterType: string(converters.CloudStorage),
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
					SourceLabelKey:    storageName,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group":   resourceGroup,
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: PullSubscription %q has not yet been reconciled", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, storageName)),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
		Objects: []runtime.Object{
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(storageName, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudStorage),
				})),
			newSink(),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
				reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
				reconcilertestingv1.WithCloudStorageSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilepullSubscriptionMsg),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: PullSubscription %q has not yet been reconciled", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, storageName)),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
		Objects: []runtime.Object{
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(storageName, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudStorage),
				}), reconcilertestingv1.WithPullSubscriptionFailed()),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
				reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
				reconcilertestingv1.WithCloudStorageSourcePullSubscriptionFailed("InvalidSink", `sinks.testing.cloud.google.com "sink" not found`),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: the status of PullSubscription %q is False", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, storageName)),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
		Objects: []runtime.Object{
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(storageName, testNS, reconcilertestingv1.WithPullSubscriptionUnknown(),
				reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudStorage),
				})),
		},
		Key: testNS + "/" + storageName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
				reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
				reconcilertestingv1.WithCloudStorageSourcePullSubscriptionUnknown("", ""),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailed, fmt.Sprintf("%s: %s: the status of PullSubscription %q is Unknown", failedToReconcilePubSubMsg, failedToPropagatePullSubscriptionStatusMsg, storageName)),
		},
	}, {
		Name: "client create fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceProject(testProject),
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
			reconcilertestingv1.NewTopic(storageName, testNS,
				reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					Project:           testProject,
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(storageName, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Project: testProject,
						Secret:  &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudStorage),
				}),
				reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
			Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
				reconcilertestingv1.WithCloudStorageSourceProject(testProject),
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
				reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
				reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
				reconcilertestingv1.WithInitCloudStorageSourceConditions,
				reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
				reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
				reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
				reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
				reconcilertestingv1.WithCloudStorageSourceSubscriptionID(reconcilertestingv1.SubscriptionID),
				reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
				reconcilertestingv1.WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "create-client-induced-error")),
				reconcilertestingv1.WithCloudStorageSourceSetDefaults,
			),
		}},
	},
		{
			Name: "bucket notifications fails",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: gcpduckv1.PubSubSpec{
							Project: testProject,
							Secret:  &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudStorage),
					}),
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithInitCloudStorageSourceConditions,
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
					reconcilertestingv1.WithCloudStorageSourceSubscriptionID(reconcilertestingv1.SubscriptionID),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-notifications-induced-error")),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		}, {
			Name: "bucket add notification fails",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: gcpduckv1.PubSubSpec{
							Project: testProject,
							Secret:  &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudStorage),
					}),
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithInitCloudStorageSourceConditions,
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
					reconcilertestingv1.WithCloudStorageSourceSubscriptionID(reconcilertestingv1.SubscriptionID),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "bucket-add-notification-induced-error")),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		}, {
			Name: "bucket doesn't exist",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: gcpduckv1.PubSubSpec{
							Project: testProject,
							Secret:  &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudStorage),
					}),
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						AttrsError: storage.ErrBucketNotExist,
					},
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", storageName),
				Eventf(corev1.EventTypeWarning, reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "storage: bucket doesn't exist")),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, storageName, true),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithInitCloudStorageSourceConditions,
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
					reconcilertestingv1.WithCloudStorageSourceSubscriptionID(reconcilertestingv1.SubscriptionID),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationNotReady(reconciledNotificationFailed, fmt.Sprintf("%s: %s", failedToReconcileNotificationMsg, "storage: bucket doesn't exist")),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		}, {
			Name: "successfully created notification",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicSpec(inteventsv1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: gcpduckv1.PubSubSpec{
							Project: testProject,
							Secret:  &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudStorage),
					}),
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceStatusObservedGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithInitCloudStorageSourceConditions,
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourceProjectID(testProject),
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
					reconcilertestingv1.WithCloudStorageSourceSubscriptionID(reconcilertestingv1.SubscriptionID),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationReady(notificationId),
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		},
		{
			Name: "delete fails with non grpc error",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationReady(notificationId),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
				Eventf(corev1.EventTypeWarning, deleteNotificationFailed, fmt.Sprintf("%s: delete-notification-induced-error", failedToDeleteNotificationMsg)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceNotificationUnknown(deleteNotificationFailed,
						"Failed from CloudStorageSource client while deleting CloudStorageSource notification: delete-notification-induced-error"),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourceNotificationID(notificationId),
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		}, {
			Name: "delete fails with Unknown grpc error",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationReady(notificationId),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
				Eventf(corev1.EventTypeWarning, deleteNotificationFailed,
					fmt.Sprintf("%s: rpc error: code = %s desc = %s", failedToDeleteNotificationMsg, codes.Unknown, "delete-notification-induced-error")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(sinkURI),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceNotificationUnknown(deleteNotificationFailed,
						fmt.Sprintf("%s: rpc error: code = %s desc = %s", failedToDeleteNotificationMsg, codes.Unknown, "delete-notification-induced-error")),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourceNotificationID(notificationId),
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		}, {
			Name: "successfully deleted storage when bucket doesn't exist",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + storageName,
			OtherTestData: map[string]interface{}{
				"storage": gstorage.TestClientData{
					BucketData: gstorage.TestBucketData{
						AttrsError: storage.ErrBucketNotExist,
					},
				},
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "reconcilertestingv1", Resource: "topics"}},
					Name: storageName,
				},
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "reconcilertestingv1", Resource: "pullsubscriptions"}},
					Name: storageName,
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceTopicDeleted,
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionDeleted,
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		},
		{
			Name: "successfully deleted storage",
			Objects: []runtime.Object{
				reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceSinkURI(storageSinkURL),
					reconcilertestingv1.WithCloudStorageSourceNotificationReady(notificationId),
					reconcilertestingv1.WithCloudStorageSourceTopicReady(testTopicID),
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionReady,
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
				reconcilertestingv1.NewTopic(storageName, testNS,
					reconcilertestingv1.WithTopicReady(testTopicID),
					reconcilertestingv1.WithTopicAddress(testTopicURI),
					reconcilertestingv1.WithTopicProjectID(testProject),
					reconcilertestingv1.WithTopicSetDefaults,
				),
				reconcilertestingv1.NewPullSubscription(storageName, testNS,
					reconcilertestingv1.WithPullSubscriptionReady(sinkURI),
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
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "reconcilertestingv1", Resource: "topics"}},
					Name: storageName,
				},
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "reconcilertestingv1", Resource: "pullsubscriptions"}},
					Name: storageName,
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilertestingv1.NewCloudStorageSource(storageName, testNS,
					reconcilertestingv1.WithCloudStorageSourceProject(testProject),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceBucket(bucket),
					reconcilertestingv1.WithCloudStorageSourceSink(sinkGVK, sinkName),
					reconcilertestingv1.WithCloudStorageSourceEventTypes([]string{schemasv1.CloudStorageObjectFinalizedEventType}),
					reconcilertestingv1.WithCloudStorageSourceObjectMetaGeneration(generation),
					reconcilertestingv1.WithCloudStorageSourceNotificationDeleted(notificationId),
					reconcilertestingv1.WithCloudStorageSourceTopicDeleted,
					reconcilertestingv1.WithCloudStorageSourcePullSubscriptionDeleted,
					reconcilertestingv1.WithDeletionTimestamp,
					reconcilertestingv1.WithCloudStorageSourceSetDefaults,
				),
			}},
		},
	}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase: intevents.NewPubSubBase(ctx,
				&intevents.PubSubBaseArgs{
					ControllerAgentName: controllerAgentName,
					ReceiveAdapterName:  receiveAdapterName,
					ReceiveAdapterType:  string(converters.CloudStorage),
					ConfigWatcher:       cmw,
				}),
			Identity:       identity.NewIdentity(ctx, NoopIAMPolicyManager, NewGCPAuthTestStore(t, nil)),
			storageLister:  listers.GetCloudStorageSourceLister(),
			createClientFn: gstorage.TestClientCreator(testData["storage"]),
		}
		return cloudstoragesource.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetCloudStorageSourceLister(), r.Recorder, r)
	}))

}
