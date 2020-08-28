/*
Copyright 2020 Google LLC

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

package keda

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	v12 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"

	gcpduck "github.com/google/knative-gcp/pkg/apis/duck"
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	pubsubv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1/resource"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1/pullsubscription"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	psreconciler "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription"
	. "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/keda/resources"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
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

	testSubscriptionID = fmt.Sprintf("cre-ps_%s_%s_%s", testNS, sourceName, sourceUID)

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1",
		Kind:    "Sink",
	}

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

func newPullSubscription(subscriptionId string) *pubsubv1.PullSubscription {
	return v12.NewPullSubscription(sourceName, testNS,
		v12.WithPullSubscriptionUID(sourceUID),
		v12.WithPullSubscriptionAnnotations(newAnnotations()),
		v12.WithPullSubscriptionObjectMetaGeneration(generation),
		v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
			Topic: testTopicID,
			PubSubSpec: gcpduckv1.PubSubSpec{
				Secret:  &secret,
				Project: testProject,
			},
		}),
		v12.WithPullSubscriptionSubscriptionID(subscriptionId),
		v12.WithInitPullSubscriptionConditions,
		v12.WithPullSubscriptionSink(sinkGVK, sinkName),
		v12.WithPullSubscriptionMarkSink(sinkURI),
		v12.WithPullSubscriptionSetDefaults,
	)
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

func newAnnotations() map[string]string {
	return map[string]string{
		gcpduck.AutoscalingClassAnnotation:                gcpduck.KEDA,
		gcpduck.AutoscalingMinScaleAnnotation:             "0",
		gcpduck.AutoscalingMaxScaleAnnotation:             "3",
		gcpduck.KedaAutoscalingSubscriptionSizeAnnotation: "5",
		gcpduck.KedaAutoscalingCooldownPeriodAnnotation:   "60",
		gcpduck.KedaAutoscalingPollingIntervalAnnotation:  "30",
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
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				},
				),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionSetDefaults,
			),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "InvalidSink",
				`InvalidSink: failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				// Updates
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionSinkNotFound(),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
	}, {
		Name: "create client fails",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionSetDefaults,
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
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				v12.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "client-create-induced-error")),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "topic exists fails",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionSetDefaults,
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
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				v12.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "topic-exists-induced-error")),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "topic does not exist",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionSetDefaults,
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
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				v12.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: Topic %q does not exist", failedToReconcileSubscriptionMsg, testTopicID)),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "subscription exists fails",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionSetDefaults,
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
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				v12.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "subscription-exists-induced-error")),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "create subscription fails",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionSetDefaults,
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
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				v12.WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "subscription-create-induced-error")),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "successfully created subscription",
		Objects: []runtime.Object{
			newPullSubscription(""),
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
			newScaledObject(newPullSubscription(testSubscriptionID)),
			newReceiveAdapter(context.Background(), testImage, nil),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				// Updates
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				v12.WithPullSubscriptionMarkNoDeployed(deploymentName(testSubscriptionID), testNS),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "successful create - reuse existing receive adapter - match",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newSecret(),
			newAvailableReceiveAdapter(context.Background(), testImage, nil),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
			},
		},
		WantCreates: []runtime.Object{
			newScaledObject(newPullSubscription(testSubscriptionID)),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "PullSubscriptionReconciled", `PullSubscription reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				v12.WithPullSubscriptionMarkDeployed(deploymentName(testSubscriptionID), testNS),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				v12.WithPullSubscriptionTransformerURI(nil),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "successful create - reuse existing receive adapter - mismatch",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				v12.WithPullSubscriptionSetDefaults,
			),
			newSink(),
			newTransformer(),
			newSecret(),
			newReceiveAdapter(context.Background(), "old"+testImage, nil),
		},
		WantCreates: []runtime.Object{
			newScaledObject(newPullSubscription(testSubscriptionID)),
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
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithInitPullSubscriptionConditions,
				v12.WithPullSubscriptionProjectID(testProject),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionTransformer(transformerGVK, transformerName),
				v12.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				v12.WithPullSubscriptionMarkNoDeployed(deploymentName(testSubscriptionID), testNS),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionMarkTransformer(transformerURI),
				v12.WithPullSubscriptionStatusObservedGeneration(generation),
				v12.WithPullSubscriptionSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, resourceGroup),
		},
	}, {
		Name: "deleting - failed to delete subscription",
		Objects: []runtime.Object{
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				v12.WithPullSubscriptionMarkDeployed(deploymentName(testSubscriptionID), testNS),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionDeleted,
				v12.WithPullSubscriptionSetDefaults,
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
			v12.NewPullSubscription(sourceName, testNS,
				v12.WithPullSubscriptionUID(sourceUID),
				v12.WithPullSubscriptionAnnotations(newAnnotations()),
				v12.WithPullSubscriptionObjectMetaGeneration(generation),
				v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret:  &secret,
						Project: testProject,
					},
					Topic: testTopicID,
				}),
				v12.WithPullSubscriptionSink(sinkGVK, sinkName),
				v12.WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				v12.WithPullSubscriptionMarkDeployed(deploymentName(testSubscriptionID), testNS),
				v12.WithPullSubscriptionMarkSink(sinkURI),
				v12.WithPullSubscriptionSubscriptionID(""),
				v12.WithPullSubscriptionDeleted,
				v12.WithPullSubscriptionSetDefaults,
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
		Key:               testNS + "/" + sourceName,
		WantEvents:        nil,
		WantStatusUpdates: nil,
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		ctx = resource.WithDuck(ctx)
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
				CreateClientFn:         gpubsub.TestClientCreator(testData["ps"]),
				ControllerAgentName:    controllerAgentName,
				ResourceGroup:          resourceGroup,
			},
		}
		r.ReconcileDataPlaneFn = r.ReconcileScaledObject
		r.scaledObjectTracker = duck.NewListableTracker(ctx, resource.Get, func(types.NamespacedName) {}, 0)
		r.discoveryFn = mockDiscoveryFunc
		return pullsubscription.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetPullSubscriptionLister(), r.Recorder, r)
	}))
}

func mockDiscoveryFunc(_ discovery.DiscoveryInterface, _ schema.GroupVersion) error {
	return nil
}

func deploymentName(subscriptionID string) string {
	ps := newPullSubscription(subscriptionID)
	return resources.GenerateReceiveAdapterName(ps)
}

func newReceiveAdapter(ctx context.Context, image string, transformer *apis.URL) runtime.Object {
	ps := v12.NewPullSubscription(sourceName, testNS,
		v12.WithPullSubscriptionUID(sourceUID),
		v12.WithPullSubscriptionAnnotations(map[string]string{
			gcpduck.AutoscalingClassAnnotation:                gcpduck.KEDA,
			gcpduck.AutoscalingMinScaleAnnotation:             "0",
			gcpduck.AutoscalingMaxScaleAnnotation:             "3",
			gcpduck.KedaAutoscalingSubscriptionSizeAnnotation: "5",
			gcpduck.KedaAutoscalingCooldownPeriodAnnotation:   "60",
			gcpduck.KedaAutoscalingPollingIntervalAnnotation:  "30",
		}),
		v12.WithPullSubscriptionSpec(pubsubv1.PullSubscriptionSpec{
			PubSubSpec: gcpduckv1.PubSubSpec{
				Secret:  &secret,
				Project: testProject,
			},
			Topic: testTopicID,
		}),
		v12.WithPullSubscriptionSetDefaults,
	)
	args := &resources.ReceiveAdapterArgs{
		Image:            image,
		PullSubscription: ps,
		Labels:           resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID:   testSubscriptionID,
		SinkURI:          sinkURI,
		TransformerURI:   transformer,
	}
	return resources.MakeReceiveAdapter(ctx, args)
}

func newAvailableReceiveAdapter(ctx context.Context, image string, transformer *apis.URL) runtime.Object {
	obj := newReceiveAdapter(ctx, image, transformer)
	ra := obj.(*v1.Deployment)
	WithDeploymentAvailable()(ra)
	return obj
}

func newScaledObject(ps *pubsubv1.PullSubscription) runtime.Object {
	ctx := context.Background()
	ra := newReceiveAdapter(ctx, testImage, nil)
	d, _ := ra.(*v1.Deployment)
	u := MakeScaledObject(ctx, d, ps)
	return u
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
