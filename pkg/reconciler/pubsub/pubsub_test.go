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

package pubsub

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

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	pubsubName = "my-test-pubsub"
	pubsubUID  = "test-pubsub-uid"
	sinkName   = "sink"

	testNS       = "testnamespace"
	testProject  = "test-project-id"
	testTopicID  = "test-topic"
	testTopicURI = "http://" + pubsubName + "-topic." + testNS + ".svc.cluster.local"
	generation   = 1
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
	topicNotReadyMsg            = "Topic testnamespace/my-test-pubsub not ready"
	pullSubscriptionNotReadyMsg = "PullSubscription testnamespace/my-test-pubsub not ready"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test PubSub object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.run/v1alpha1",
		Kind:               "PubSub",
		Name:               pubsubName,
		UID:                pubsubUID,
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

// turn string into URL or terminate with t.Fatalf
func sinkURL(t *testing.T, url string) *apis.URL {
	u, err := apis.ParseURL(url)
	if err != nil {
		t.Fatalf("Failed to parse url %q", url)
	}
	return u
}

func TestAllCases(t *testing.T) {
	pubsubSinkURL := sinkURL(t, sinkURI)

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
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithInitPubSubConditions,
				WithPubSubTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(pubsubName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter": "pubsub.events.cloud.run",
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, pubsubName, true),
		},
	}, {
		Name: "topic exists, topic not ready",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicTopicID(testTopicID),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopicNotReady("TopicNotReady", "Topic testnamespace/my-test-pubsub did not expose projectid"),
				WithPubSubFinalizers(finalizerName),
			),
		}},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopicNotReady("TopicNotReady", "Topic testnamespace/my-test-pubsub did not expose topicid"),
				WithPubSubFinalizers(finalizerName),
			),
		}},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopicNotReady("TopicNotReady", `Topic testnamespace/my-test-pubsub topic mismatch expected "test-topic" got "garbaaaaage"`),
				WithPubSubFinalizers(finalizerName),
				WithPubSubStatusObservedGeneration(generation),
			),
		}},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopicReady(testTopicID),
				WithPubSubProjectID(testProject),
				WithPubSubFinalizers(finalizerName),
				WithPubSubPullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(pubsubName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic:  testTopicID,
					Secret: &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": "pubsub.events.cloud.run",
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": "pubsubs.events.cloud.run",
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but is not ready",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(pubsubName, testNS),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopicReady(testTopicID),
				WithPubSubProjectID(testProject),
				WithPubSubPullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewTopic(pubsubName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(pubsubName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + pubsubName,
		WantErr: false,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
				WithInitPubSubConditions,
				WithPubSubTopicReady(testTopicID),
				WithPubSubPullSubscriptionReady(),
				WithPubSubSinkURI(pubsubSinkURL),
				WithPubSubProjectID(testProject),
			),
		}},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			PubSubBase:   reconciler.NewPubSubBase(ctx, controllerAgentName, "pubsub.events.cloud.run", cmw),
			pubsubLister: listers.GetPubSubLister(),
		}
	}))

}
