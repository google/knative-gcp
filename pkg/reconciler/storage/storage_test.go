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

	//	"knative.dev/pkg/apis/duck/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	//	"k8s.io/apimachinery/pkg/util/intstr"
	//	"knative.dev/pkg/apis"
	//	"knative.dev/pkg/apis/duck/v1beta1"
	//	"knative.dev/pkg/kmeta"

	//	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	//	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	storagev1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	pubsubClient "github.com/google/knative-gcp/pkg/client/injection/client"
	//	ops "github.com/google/knative-gcp/pkg/operations"
	//	operations "github.com/google/knative-gcp/pkg/operations/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
	//	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	//	"github.com/google/knative-gcp/pkg/reconciler/storage/resources"

	. "knative.dev/pkg/reconciler/testing"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	storageName = "my-test-storage"
	bucket      = "my-test-bucket"
	sinkName    = "sink"

	testNS       = "testnamespace"
	testImage    = "notification-ops-image"
	topicUID     = storageName + "-abc-123"
	topicName    = "storage-test-storage-uid"
	testProject  = "test-project-id"
	testTopicID  = "cloud-run-topic-" + testNS + "-" + storageName + "-" + topicUID
	testTopicURI = "http://" + storageName + "-topic." + testNS + ".svc.cluster.local"
)

var (
	trueVal = true

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = "http://" + sinkDNS + "/"

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.run",
		Version: "v1alpha1",
		Kind:    "Sink",
	}

	secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "google-cloud-key",
		},
		Key: "key.json",
	}

	// Message for when the topic and pullsubscription with the above variables is not ready.
	topicNotReadyMsg            = "Topic testnamespace/my-test-storage not ready"
	pullSubscriptionNotReadyMsg = "PullSubscription testnamespace/my-test-storage not ready"
)

func init() {
	// Add types to scheme
	_ = storagev1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test Storage object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.run/v1alpha1",
		Kind:               "Storage",
		Name:               "my-test-storage",
		UID:                "test-storage-uid",
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
			"apiVersion": "testing.cloud.run/v1alpha1",
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
		Name: "topic created, not ready",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(storageName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             topicName,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter": "storage.events.cloud.run",
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
	}, {
		Name: "topic exists, topic not ready",
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
			),
			NewTopic(storageName, testNS,
				WithTopicTopicID(topicName),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithStorageFinalizers(finalizerName),
				WithInitStorageConditions,
				WithStorageTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
	}, {
		Objects: []runtime.Object{
			NewStorage(storageName, testNS,
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
			),
			NewTopic(storageName, testNS,
				WithTopicReady(topicName),
				WithTopicAddress("http://test-example.com"),
			),
			newSink(),
		},
		Key:     testNS + "/" + storageName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewStorage(storageName, testNS,
				WithStorageBucket(bucket),
				WithStorageSink(sinkGVK, sinkName),
				WithInitStorageConditions,
				WithStorageTopicReady(),
				WithStoragePullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(storageName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic:  topicName,
					Secret: &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": "storage.events.cloud.run",
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, storageName, true),
		},
		/*
			}, {
				Name: "create topic",
				Objects: []runtime.Object{
					NewTopic(topicName, testNS,
						WithTopicUID(topicUID),
						WithTopicSpec(pubsubv1alpha1.TopicSpec{
							Project: testProject,
							Topic:   testTopicID,
							Secret:  &secret,
						}),
					),
					newSink(),
				},
				Key: testNS + "/" + topicName,
				WantEvents: []string{
					Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
					Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
					Object: NewTopic(topicName, testNS,
						WithTopicUID(topicUID),
						WithTopicSpec(pubsubv1alpha1.TopicSpec{
							Project: testProject,
							Topic:   testTopicID,
							Secret:  &secret,
						}),
						// Updates
						WithInitTopicConditions,
						WithTopicMarkTopicCreating(testTopicID),
					),
				}},
				WantCreates: []runtime.Object{
					newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionCreate),
				},
				WantPatches: []clientgotesting.PatchActionImpl{
					patchFinalizers(testNS, topicName, true),
				},
			}, {
		*/
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			NotificationOpsImage: testImage,
			Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
			storageLister:        listers.GetStorageLister(),
			pubsubClient:         pubsubClient.Get(ctx),
			jobLister:            listers.GetJobLister(),
		}
	}))

}
