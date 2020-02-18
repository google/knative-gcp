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

package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
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
	sinkURI = "http://" + sinkDNS

	transformerDNS = transformerName + ".mynamespace.svc.cluster.local"
	transformerURI = "http://" + transformerDNS

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
					"url": sinkURI,
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
					"url": transformerURI,
				},
			},
		},
	}
}

func TestAllCases(t *testing.T) {
	attempts := 0
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
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
			),
			newSecret(),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError",
				`failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				// updates
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSinkNotFound(),
			),
		}},
	}, {
		Name: "create client fails",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "InternalError", "client-create-induced-error"),
		},
		WantErr: true,
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				CreateClientErr: errors.New("client-create-induced-error"),
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "client-create-induced-error"))),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, finalizerName),
		},
	}, {
		Name: "topic exists fails",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "topic-exists-induced-error"),
		},
		WantErr: true,
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					ExistsErr: errors.New("topic-exists-induced-error"),
				},
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "topic-exists-induced-error"))),
		}},
	}, {
		Name: "topic does not exist",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q does not exist", testTopicID),
		},
		WantErr: true,
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: false,
				},
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: Topic %q does not exist", failedToReconcileSubscriptionMsg, testTopicID))),
		}},
	}, {
		Name: "subscription exists fails",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "subscription-exists-induced-error"),
		},
		WantErr: true,
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				SubscriptionData: gpubsub.TestSubscriptionData{
					ExistsErr: errors.New("subscription-exists-induced-error"),
				},
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "subscription-exists-induced-error"))),
		}},
	}, {
		Name: "create subscription fails",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "subscription-create-induced-error"),
		},
		WantErr: true,
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
				CreateSubscriptionErr: errors.New("subscription-create-induced-error"),
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				WithPullSubscriptionMarkNoSubscription("SubscriptionReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileSubscriptionMsg, "subscription-create-induced-error"))),
		}},
	}, {
		Name: "successfully created subscription",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "ReadinessChanged", "PullSubscription %q became ready", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
			},
		},
		WantCreates: []runtime.Object{
			newReceiveAdapter(context.Background(), testImage),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				// Updates
				WithPullSubscriptionStatusObservedGeneration(generation),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, finalizerName),
		},
	}, {
		Name: "successful create - reuse existing receive adapter - match",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
			),
			newSink(),
			newSecret(),
			newReceiveAdapter(context.Background(), testImage),
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
			Eventf(corev1.EventTypeNormal, "ReadinessChanged", "PullSubscription %q became ready", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoTransformer("TransformerNil", "Transformer is nil"),
				WithPullSubscriptionTransformerURI(""),
				WithPullSubscriptionStatusObservedGeneration(generation),
			),
		}},
	}, {
		Name: "successful create - reuse existing receive adapter - mismatch, with retry",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionTransformer(transformerGVK, transformerName),
			),
			newSink(),
			newTransformer(),
			newSecret(),
			newReceiveAdapter(context.Background(), "old"+testImage),
		},
		OtherTestData: map[string]interface{}{
			"ps": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					Exists: true,
				},
			},
		},
		Key: testNS + "/" + sourceName,
		WithReactors: []clientgotesting.ReactionFunc{
			func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				if attempts != 0 || !action.Matches("update", "pullsubscriptions") {
					return false, nil, nil
				}
				attempts++
				return true, nil, apierrs.NewConflict(v1alpha1.GroupResource("foo"), "bar", errors.New("foo"))
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ReadinessChanged", "PullSubscription %q became ready", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
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
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionTransformer(transformerGVK, transformerName),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkTransformer(transformerURI),
				WithPullSubscriptionStatusObservedGeneration(generation),
			),
		}, {
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionProjectID(testProject),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionTransformer(transformerGVK, transformerName),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkTransformer(transformerURI),
				WithPullSubscriptionStatusObservedGeneration(generation),
			),
		}},
	}, {
		Name: "deleting - failed to delete subscription",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionDeleted,
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
			Eventf(corev1.EventTypeWarning, "InternalError", "subscription-delete-induced-error"),
		},
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkNoSubscription("SubscriptionDeleteFailed", fmt.Sprintf("%s: %s", failedToDeleteSubscriptionMsg, "subscription-delete-induced-error")),
				WithPullSubscriptionSubscriptionID(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionDeleted,
			),
		}},
	}, {
		Name: "successfully deleted subscription",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionDeleted,
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
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithPullSubscriptionUID(sourceUID),
				WithPullSubscriptionFinalizers(finalizerName),
				WithPullSubscriptionObjectMetaGeneration(generation),
				WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkNoSubscription("SubscriptionDeleted", fmt.Sprintf("Successfully deleted Pub/Sub subscription %q", testSubscriptionID)),
				WithPullSubscriptionMarkDeployed,
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionStatusObservedGeneration(generation),
				WithPullSubscriptionDeleted,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, ""),
		},
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
				PullSubscriptionLister: listers.GetPullSubscriptionLister(),
				UriResolver:            resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
				ReceiveAdapterImage:    testImage,
				CreateClientFn:         gpubsub.TestClientCreator(testData["ps"]),
				ControllerAgentName:    controllerAgentName,
				FinalizerName:          finalizerName,
			},
		}
		r.ReconcileDataPlaneFn = r.ReconcileDeployment
		return r
	}))
}

func newReceiveAdapter(ctx context.Context, image string, transformer ...string) runtime.Object {
	source := NewPullSubscription(sourceName, testNS,
		WithPullSubscriptionUID(sourceUID),
		WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
			Project: testProject,
			Topic:   testTopicID,
			Secret:  &secret,
		}))
	transformerURI := ""
	if len(transformer) > 0 {
		transformerURI = transformer[0]
	}
	args := &resources.ReceiveAdapterArgs{
		Image:          image,
		Source:         source,
		Labels:         resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID: testSubscriptionID,
		SinkURI:        sinkURI,
		TransformerURI: transformerURI,
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
