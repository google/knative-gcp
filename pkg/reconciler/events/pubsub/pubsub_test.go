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
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test PubSub object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "PubSub",
		Name:               pubsubName,
		UID:                pubsubUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
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
		Name: "pullsubscription created",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			newSink(),
		},
		Key: testNS + "/" + pubsubName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubFinalizers(finalizerName),
				WithPubSubPullSubscriptionNotReady("PullSubscriptionNotReady", "PullSubscription has no Ready type status"),
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
					"receive-adapter": "pubsub.events.cloud.google.com",
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": "pubsubs.events.cloud.google.com",
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		Name: "pullsubscription exists but is not ready",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewPullSubscriptionWithNoDefaults(pubsubName, testNS,
				WithPullSubscriptionReadyStatus(corev1.ConditionFalse, "PullSubscriptionNotReady", "no ready test message")),
			newSink(),
		},
		Key: testNS + "/" + pubsubName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
				WithInitPubSubConditions,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionNotReady("PullSubscriptionNotReady", "no ready test message"),
			),
		}},
	}, {
		Name: "pullsubscription exists and ready",
		Objects: []runtime.Object{
			NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
			),
			NewPullSubscriptionWithNoDefaults(pubsubName, testNS,
				WithPullSubscriptionReady(sinkURI),
				WithPullSubscriptionReadyStatus(corev1.ConditionTrue, "PullSubscriptionNoReady", ""),
			),
			newSink(),
		},
		Key: testNS + "/" + pubsubName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSub(pubsubName, testNS,
				WithPubSubObjectMetaGeneration(generation),
				WithPubSubStatusObservedGeneration(generation),
				WithPubSubTopic(testTopicID),
				WithPubSubSink(sinkGVK, sinkName),
				WithPubSubFinalizers(finalizerName),
				WithInitPubSubConditions,
				WithPubSubPullSubscriptionReady(),
				WithPubSubSinkURI(pubsubSinkURL),
			),
		}},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                   reconciler.NewBase(ctx, controllerAgentName, cmw),
			pubsubLister:           listers.GetPubSubLister(),
			pullsubscriptionLister: listers.GetPullSubscriptionLister(),
			receiveAdapterName:     "pubsub.events.cloud.google.com",
		}
	}))

}
