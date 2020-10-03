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
	"fmt"
	"strings"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	v1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"

	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	pubsubv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1/pullsubscription"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	psreconciler "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
)

const (
	sourceName      = "source"
	sinkName        = "sink"
	transformerName = "transformer"

	testNS = "testnamespace"

	testImage = "test_image"

	sourceUID = sourceName + "-abc-123"

	testProject = "test-project-id"
	testTopicID = sourceUID + "-TOPIC"
	generation  = 1

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
		Version: "v1",
		Kind:    "Sink",
	}

	testSubscriptionID = fmt.Sprintf("cre-ps_%s_%s_%s", testNS, sourceName, sourceUID)

	transformerGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1",
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
	_ = pubsubv1.AddToScheme(scheme.Scheme)
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
			"apiVersion": "testing.cloud.google.com/v1",
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

func newSinkDestination(namespace string) duckv1.Destination {
	return duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "testing.cloud.google.com/v1",
			Kind:       "Sink",
			Name:       sinkName,
			Namespace:  namespace,
		},
	}
}

func newTransformer() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testing.cloud.google.com/v1",
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
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "InvalidSink",
				`InvalidSink: sinks.testing.cloud.google.com "sink" not found`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				// updates
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSinkNotFound(),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
	}, {
		Name: "create client fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
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
			"client-error": "client-create-induced-error",
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				reconcilertestingv1.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "client-create-induced-error")),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "topic exists fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: rpc error: code = Internal desc = Injected error"),
		},
		OtherTestData: map[string]interface{}{
			// GetTopic has a retry policy for Unknown status type, so we use Internal error instead.
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("GetTopic", codes.Internal, "Injected error")},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				reconcilertestingv1.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "rpc error: code = Internal desc = Injected error")),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			NoSubscriptionsExist(),
		},
	}, {
		Name: "topic does not exist",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: Topic %q does not exist", testTopicID),
		},
		OtherTestData: map[string]interface{}{},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				reconcilertestingv1.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: Topic %q does not exist", failedToReconcileSubscriptionMsg, testTopicID)),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			NoSubscriptionsExist(),
		},
	}, {
		Name: "subscription exists fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: rpc error: code = Internal desc = Injected error"),
		},
		OtherTestData: map[string]interface{}{
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("GetSubscription", codes.Internal, "Injected error")},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				reconcilertestingv1.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "rpc error: code = Internal desc = Injected error")),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
	}, {
		Name: "create subscription fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "SubscriptionReconcileFailed", "Failed to reconcile Pub/Sub subscription: rpc error: code = Internal desc = Injected error"),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("CreateSubscription", codes.Internal, "Injected error")},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				reconcilertestingv1.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "rpc error: code = Internal desc = Injected error")),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
	}, {
		Name: "successfully created subscription",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
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
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		WantCreates: []runtime.Object{
			newReceiveAdapter(context.Background(), testImage, nil),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				// Updates
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkNoDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "sink namespace empty, default to the source one",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(""),
						},
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
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
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		WantCreates: []runtime.Object{
			newReceiveAdapter(context.Background(), testImage, nil),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				// Updates
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkNoDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "sink URI set instead of ref",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
						SourceSpec: duckv1.SourceSpec{
							Sink: duckv1.Destination{
								URI: sinkURI,
							},
						},
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
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
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		WantCreates: []runtime.Object{
			newReceiveAdapter(context.Background(), testImage, nil),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				// Updates
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkNoDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "successful create - reuse existing receive adapter - match",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newSecret(),
			newAvailableReceiveAdapter(context.Background(), testImage, nil),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
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
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				reconcilertestingv1.WithPullSubscriptionTransformerURI(nil),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "successful create - reuse existing receive adapter - mismatch",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newTransformer(),
			newSecret(),
			newReceiveAdapter(context.Background(), "old"+testImage, nil),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
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
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				//WithPullSubscriptionFinalizers(resourceGroup),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkNoDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkTransformer(transformerURI),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "get existing receiver adapter fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newTransformer(),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "DataPlaneReconcileFailed", "Failed to reconcile Data Plane resource(s): %s", "inducing failure for list deployments"),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("list", "deployments"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkDeployedUnknown("ReceiveAdapterGetFailed", "Error getting the Receive Adapter: inducing failure for list deployments"),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkTransformer(transformerURI),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "create receiver adapter fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newTransformer(),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "DataPlaneReconcileFailed", "Failed to reconcile Data Plane resource(s): %s", "inducing failure for create deployments"),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "deployments"),
		},
		WantCreates: []runtime.Object{
			newReceiveAdapter(context.Background(), testImage, transformerURI),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkDeployedFailed("ReceiveAdapterCreateFailed", "Error creating the Receive Adapter: inducing failure for create deployments"),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkTransformer(transformerURI),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "update receiver adapter fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newTransformer(),
			newSecret(),
			newReceiveAdapter(context.Background(), "old"+testImage, nil),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "DataPlaneReconcileFailed", "Failed to reconcile Data Plane resource(s): %s", "inducing failure for update deployments"),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "deployments"),
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
			Object: reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithInitPullSubscriptionConditions,
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkDeployedFailed("ReceiveAdapterUpdateFailed", "Error updating the Receive Adapter: inducing failure for update deployments"),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionMarkTransformer(transformerURI),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			OnlySubscriptions(testSubscriptionID),
		},
	}, {
		Name: "deleting - failed to delete subscription",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionDeleted,
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub(testTopicID, testSubscriptionID),
			},
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("DeleteSubscription", codes.Unknown, "Injected error")},
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", "Failed to delete Pub/Sub subscription: rpc error: code = Unknown desc = Injected error"),
		},
		WantStatusUpdates: nil,
	}, {
		Name: "successfully deleted subscription",
		Objects: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(sourceName, testNS,
				reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
				reconcilertestingv1.WithPullSubscriptionObjectMetaGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionStatusObservedGeneration(generation),
				reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				reconcilertestingv1.WithPullSubscriptionSink(sinkGVK, sinkName),
				reconcilertestingv1.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				reconcilertestingv1.WithPullSubscriptionMarkDeployed(deploymentName(), testNS),
				reconcilertestingv1.WithPullSubscriptionMarkSink(sinkURI),
				reconcilertestingv1.WithPullSubscriptionProjectID(testProject),
				reconcilertestingv1.WithPullSubscriptionDeleted,
				reconcilertestingv1.WithPullSubscriptionSetDefaults,
			),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub(testTopicID, testSubscriptionID),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoSubscriptionsExist(),
		},
		Key:        testNS + "/" + sourceName,
		WantEvents: nil,
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		opts := []pstest.ServerReactorOption{}
		if testData != nil && testData["server-options"] != nil {
			opts = testData["server-options"].([]pstest.ServerReactorOption)
		}

		srv := pstest.NewServer(opts...)

		psclient, _ := GetTestClientCreateFunc(srv.Addr)(ctx, testProject)
		conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
		if err != nil {
			panic(fmt.Errorf("failed to dial test pubsub connection: %v", err))
		}
		close := func() {
			srv.Close()
			conn.Close()
		}
		t.Cleanup(close)
		if testData != nil {
			InjectPubsubClient(testData, psclient)
			if testData["pre"] != nil {
				fixtures := testData["pre"].([]PubsubAction)
				for _, f := range fixtures {
					f(ctx, t, psclient)
				}
			}
		}
		// use normal create function or always error one
		var createClientFn reconcilerutilspubsub.CreateFn
		if testData != nil && testData["client-error"] != nil {
			createClientFn = func(ctx context.Context, projectID string, opts ...option.ClientOption) (*pubsub.Client, error) {
				return nil, fmt.Errorf(testData["client-error"].(string))
			}
		} else {
			createClientFn = GetTestClientCreateFunc(srv.Addr)
		}
		pubsubBase := &intevents.PubSubBase{
			Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
		}
		r := &Reconciler{
			Base: &psreconciler.Base{
				PubSubBase:             pubsubBase,
				DeploymentLister:       listers.GetDeploymentLister(),
				PullSubscriptionLister: listers.GetPullSubscriptionLister(),
				UriResolver:            resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
				ReceiveAdapterImage:    testImage,
				CreateClientFn:         createClientFn,
				ControllerAgentName:    controllerAgentName,
				ResourceGroup:          resourceGroup,
			},
		}
		r.ReconcileDataPlaneFn = r.ReconcileDeployment
		return pullsubscription.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetPullSubscriptionLister(), r.Recorder, r)
	}))
}

func deploymentName() string {
	ps := newPullSubscription()
	return resources.GenerateReceiveAdapterName(ps)
}

func newReceiveAdapter(ctx context.Context, image string, transformer *apis.URL) runtime.Object {
	ps := newPullSubscription()
	args := &resources.ReceiveAdapterArgs{
		Image:            image,
		PullSubscription: ps,
		Labels:           resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID:   testSubscriptionID,
		SinkURI:          sinkURI,
		TransformerURI:   transformer,
	}
	ra := resources.MakeReceiveAdapter(ctx, args)
	return ra
}

func newAvailableReceiveAdapter(ctx context.Context, image string, transformer *apis.URL) runtime.Object {
	obj := newReceiveAdapter(ctx, image, transformer)
	ra := obj.(*v1.Deployment)
	WithDeploymentAvailable()(ra)
	return obj
}

func newPullSubscription() *pubsubv1.PullSubscription {
	return reconcilertestingv1.NewPullSubscription(sourceName, testNS,
		reconcilertestingv1.WithPullSubscriptionUID(sourceUID),
		reconcilertestingv1.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
			PubSubSpec: gcpduckv1.PubSubSpec{
				Secret:  &secret,
				Project: testProject,
			},
			Topic: testTopicID,
		}),
		reconcilertestingv1.WithPullSubscriptionSetDefaults,
	)
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
