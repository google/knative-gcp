/*
Copyright 2019 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package static

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"

	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/pubsub/v1alpha1/pullsubscription"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	psreconciler "github.com/google/knative-gcp/pkg/reconciler/pubsub/pullsubscription"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/pullsubscription/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	sourceName      = "source"
	sinkName        = "sink"
	transformerName = "transformer"

	testNS = "testnamespace"

	testImage = "test_image"

	sourceUID = sourceName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = sourceUID + "-TOPIC"
	testSubscriptionID = "cre-pull-" + sourceUID
	generation         = 1

	secretName = "testing-secret"

	failedToReconcileSubscriptionMsg = `Failed to reconcile Pub/Sub subscription`
	failedToDeleteSubscriptionMsg    = `Failed to delete Pub/Sub subscription`
)

var (
	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = apis.HTTP(sinkDNS)

	transformerDNS = transformerName + ".mynamespace.svc.cluster.local"
	transformerURI = apis.HTTP(transformerDNS)

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "Sink",
	}

	transformerGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "Transformer",
	}

	secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secretName,
		},
		Key: "testing-key",
	}
)

func init() {
	// Add types to scheme
	_ = pubsubv1alpha1.AddToScheme(scheme.Scheme)
}

func newSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      secretName,
		},
		Data: map[string][]byte{
			"testing-key": []byte("abcd"),
		},
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
					"url": sinkURI.String(),
				},
			},
		},
	}
}

func newTransformer() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testing.cloud.google.com/v1alpha1",
			"kind":       "Transformer",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      transformerName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": transformerURI.String(),
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
		Name: "cannot get sink",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
			),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "InvalidSink",
				`InvalidSink: failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				// updates
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSinkNotFound(),
			),
		}},
	}, {
		Name: "create client fails",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: client-create-induced-error"),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				CreateClientErr: errors.New("client-create-induced-error"),
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				WithPubSubPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "client-create-induced-error"))),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "topic exists fails",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: topic-exists-induced-error"),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					ExistsErr: errors.New("topic-exists-induced-error"),
				},
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				WithPubSubPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "topic-exists-induced-error"))),
		}},
	}, {
		Name: "topic does not exist",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: Topic %q does not exist", testTopicID),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: false,
				},
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				WithPubSubPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: Topic %q does not exist", failedToReconcileSubscriptionMsg, testTopicID))),
		}},
	}, {
		Name: "subscription exists fails",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: subscription-exists-induced-error"),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				SubscriptionData: gpubsub.TestSubscriptionData{
					ExistsErr: errors.New("subscription-exists-induced-error"),
				},
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				WithPubSubPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "subscription-exists-induced-error"))),
		}},
	}, {
		Name: "create subscription fails",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: subscription-create-induced-error"),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
				CreateSubscriptionErr: errors.New("subscription-create-induced-error"),
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				WithPubSubPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "subscription-create-induced-error"))),
		}},
	}, {
		Name: "successfully created subscription",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "PullSubscriptionReconciled", `PullSubscription reconciled: "%s/%s"`, testNS, sourceName),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
			},
		},
		WantCreates: []runtime.Object{
			newReceiveAdapter(context.Background(), testImage, nil),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				// Updates
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPubSubPullSubscriptionMarkDeployed,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "successful create - reuse existing receive adapter - match",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
			),
			newSink(),
			newSecret(),
			newReceiveAdapter(context.Background(), testImage, nil),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
			},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "PullSubscriptionReconciled", `PullSubscription reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPubSubPullSubscriptionMarkDeployed,
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPubSubPullSubscriptionTransformerURI(nil),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
			),
		}},
	}, {
		Name: "successful create - reuse existing receive adapter - mismatch",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionTransformer(transformerGVK, transformerName),
			),
			newSink(),
			newTransformer(),
			newSecret(),
			newReceiveAdapter(context.Background(), "old"+testImage, nil),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
			},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "PullSubscriptionReconciled", `PullSubscription reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS,
				Verb:      "update",
				Resource:  receiveAdapterGVR(),
			},
			Object: newReceiveAdapter(context.Background(), testImage, transformerURI),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				//WithPullSubscriptionFinalizers(resourceGroup),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubInitPullSubscriptionConditions,
				WithPubSubPullSubscriptionProjectID(testProject),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionTransformer(transformerGVK, transformerName),
				WithPubSubPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPubSubPullSubscriptionMarkDeployed,
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionMarkTransformer(transformerURI),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
			),
		}},
	}, {
		Name: "deleting - failed to delete subscription",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPubSubPullSubscriptionMarkDeployed,
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionDeleted,
			),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
				SubscriptionData: gpubsub.TestSubscriptionData{
					Exists:    true,
					DeleteErr: errors.New("subscription-delete-induced-error"),
				},
			},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", "Failed to delete Pub/Sub subscription: subscription-delete-induced-error"),
		},
		WantStatusUpdates: nil,
	}, {
		Name: "successfully deleted subscription",
		Objects: []runtime.Object{
			NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPubSubPullSubscriptionMarkDeployed,
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionDeleted,
			),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
				SubscriptionData: gpubsub.TestSubscriptionData{
					Exists: true,
				},
			},
		},
		Key:        testNS + "/" + sourceName,
		WantEvents: nil,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubPullSubscription(sourceName, testNS,
				WithPubSubPullSubscriptionUID(sourceUID),
				WithPubSubPullSubscriptionObjectMetaGeneration(generation),
				WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				WithPubSubPullSubscriptionSink(sinkGVK, sinkName),
				WithPubSubPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPubSubPullSubscriptionSubscriptionID(""),
				WithPubSubPullSubscriptionMarkDeployed,
				WithPubSubPullSubscriptionMarkSink(sinkURI),
				WithPubSubPullSubscriptionStatusObservedGeneration(generation),
				WithPubSubPullSubscriptionDeleted,
			),
		}},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		pubsubBase := &pubsub.PubSubBase{
			Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
		}
		r := &Reconciler{
			Base: &psreconciler.Base{
				PubSubBase:             pubsubBase,
				DeploymentLister:       listers.GetDeploymentLister(),
				PullSubscriptionLister: listers.GetPubSubPullSubscriptionLister(),
				UriResolver:            resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
				ReceiveAdapterImage:    testImage,
				CreateClientFn:         gpubsub.TestClientCreator(testData["ps"]),
				ControllerAgentName:    controllerAgentName,
				ResourceGroup:          resourceGroup,
			},
		}
		r.ReconcileDataPlaneFn = r.ReconcileDeployment
		return pullsubscription.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetPubSubPullSubscriptionLister(), r.Recorder, r)
	}))
}

func newReceiveAdapter(ctx context.Context, image string, transformer *apis.URL) runtime.Object {
	source := NewPubSubPullSubscription(sourceName, testNS,
		WithPubSubPullSubscriptionUID(sourceUID),
		WithPubSubPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
			PubSubSpec: duckv1alpha1.PubSubSpec{
				Secret:  &secret,
				Project: testProject,
			},
			Topic: testTopicID,
		}))
	args := &resources.ReceiveAdapterArgs{
		Image:          image,
		Source:         source,
		Labels:         resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID: testSubscriptionID,
		SinkURI:        sinkURI,
		TransformerURI: transformer,
	}
	return resources.MakeReceiveAdapter(ctx, args)
}

func receiveAdapterGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployment",
	}
}

func patchFinalizers(namespace, name, finalizer string, existingFinalizers ...string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	for i, ef := range existingFinalizers {
		existingFinalizers[i] = fmt.Sprintf("%q", ef)
	}
	if finalizer != "" {
		existingFinalizers = append(existingFinalizers, fmt.Sprintf("%q", finalizer))
	}
	fname := strings.Join(existingFinalizers, ",")
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
