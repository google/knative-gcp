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

package pullsubscription

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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"

	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/pullsubscription/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	sourceName = "source"
	sinkName   = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	sourceUID = sourceName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = sourceUID + "-TOPIC"
	testSubscriptionID = "cre-pull-" + sourceUID
	generation         = 1

	secretName = "testing-secret"

	failedToCreateSubscriptionMsg = `Failed to create Pub/Sub subscription`
	failedToDeleteSubscriptionMsg = `Failed to delete Pub/Sub subscription`
)

var (
	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = "http://" + sinkDNS

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "Sink",
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
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoSubscription("SubscriptionCreateFailed", fmt.Sprintf("%s: %s", failedToCreateSubscriptionMsg, "client-create-induced-error"))),
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
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoSubscription("SubscriptionCreateFailed", fmt.Sprintf("%s: %s", failedToCreateSubscriptionMsg, "topic-exists-induced-error"))),
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
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoSubscription("SubscriptionCreateFailed", fmt.Sprintf("%s: Topic %q does not exist", failedToCreateSubscriptionMsg, testTopicID))),
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
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoSubscription("SubscriptionCreateFailed", fmt.Sprintf("%s: %s", failedToCreateSubscriptionMsg, "subscription-exists-induced-error"))),
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
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				WithPullSubscriptionMarkNoSubscription("SubscriptionCreateFailed", fmt.Sprintf("%s: %s", failedToCreateSubscriptionMsg, "subscription-create-induced-error"))),
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
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionMarkSink(sinkURI),
				// Updates
				WithPullSubscriptionStatusObservedGeneration(generation),
				WithPullSubscriptionMarkSubscribed(testSubscriptionID),
				WithPullSubscriptionMarkDeployed,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, finalizerName),
		},
	},
	//{
	//	Name: "successful create",
	//	Objects: []runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//		),
	//		newSink(),
	//		newSecret(true),
	//		//newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionCreate),
	//	},
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			// Updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithInitPullSubscriptionConditions,
	//			WithPullSubscriptionReady(sinkURI),
	//		),
	//	}},
	//	WantCreates: []runtime.Object{
	//		newReceiveAdapter(context.Background(), testImage),
	//	},
	//}, {
	//	Name: "successful create - reuse existing receive adapter - match",
	//	Objects: []runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//		),
	//		newSink(),
	//		newSecret(true),
	//		newReceiveAdapter(context.Background(), testImage),
	//		//newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionCreate),
	//	},
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			// Updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithInitPullSubscriptionConditions,
	//			WithPullSubscriptionReady(sinkURI),
	//		),
	//	}},
	//}, {
	//	Name: "successful create - reuse existing receive adapter - mismatch",
	//	Objects: []runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//		),
	//		newSink(),
	//		newSecret(true),
	//		newReceiveAdapter(context.Background(), "old"+testImage),
	//		//newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionCreate),
	//	},
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantUpdates: []clientgotesting.UpdateActionImpl{{
	//		ActionImpl: clientgotesting.ActionImpl{
	//			Namespace: testNS,
	//			Verb:      "update",
	//			Resource:  receiveAdapterGVR(),
	//		},
	//		Object: newReceiveAdapter(context.Background(), testImage),
	//	}},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			// Updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithInitPullSubscriptionConditions,
	//			WithPullSubscriptionReady(sinkURI),
	//		),
	//	}},
	//}, {
	//	Name: "fail to create subscription",
	//	Objects: append([]runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithInitPullSubscriptionConditions,
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionMarkSink(sinkURI),
	//		),
	//		newSink()},
	//	//newJobFinished(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionCreate, false)...,
	//	),
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeWarning, "InternalError", testJobFailureMessage),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithInitPullSubscriptionConditions,
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionMarkSink(sinkURI),
	//			// Updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithPullSubscriptionJobFailure(testSubscriptionID, "CreateFailed", fmt.Sprintf("Failed to create Subscription: %q.", testJobFailureMessage)),
	//		),
	//	}},
	//	WantErr: true,
	//},
	//{
	//	Name: "deleting - delete subscription",
	//	Objects: []runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//			WithPullSubscriptionDeleted,
	//		),
	//		newSecret(true),
	//	},
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//			WithPullSubscriptionDeleted,
	//			// updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//		),
	//	}},
	//	WantCreates: []runtime.Object{
	//		//newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionDelete),
	//	},
	//},
	//{
	//	Name: "deleting final stage",
	//	Objects: append([]runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//		),
	//		newSecret(true)},
	//	//newJobFinished(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionDelete, true)...,
	//	),
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//			// updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithPullSubscriptionMarkNoSubscription(testSubscriptionID),
	//		),
	//	}},
	//	WantPatches: []clientgotesting.PatchActionImpl{
	//		patchFinalizers(testNS, sourceName, ""),
	//		patchFinalizers(testNS, secretName, "", "noisy-finalizer"),
	//	},
	//},
	//{
	//	Name: "skip deleting if subscription not exists",
	//	Objects: []runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionJobFailure(testSubscriptionID, "CreateFailed", "subscription creation failed"),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionFinalizers(finalizerName),
	//		),
	//		newSecret(true),
	//	},
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionDeleted,
	//			WithInitPullSubscriptionConditions,
	//			WithPullSubscriptionJobFailure(testSubscriptionID, "CreateFailed", "subscription creation failed"),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//			// updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//		),
	//	}},
	//	WantPatches: []clientgotesting.PatchActionImpl{
	//		patchFinalizers(testNS, sourceName, ""),
	//		patchFinalizers(testNS, secretName, "", "noisy-finalizer"),
	//	},
	//},
	//{
	//	Name: "deleting final stage - not the only PullSubscription",
	//	Objects: append([]runtime.Object{
	//		NewPullSubscription("not-relevant", testNS,
	//			WithPullSubscriptionUID("not-relevant"),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//		),
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//		),
	//		newSecret(true)},
	//	//newJobFinished(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionDelete, true)...,
	//	),
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
	//		Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//			// updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithPullSubscriptionMarkNoSubscription(testSubscriptionID),
	//		),
	//	}},
	//	WantPatches: []clientgotesting.PatchActionImpl{
	//		patchFinalizers(testNS, sourceName, ""),
	//	},
	//},
	//{
	//	Name: "fail to delete subscription",
	//	Objects: append([]runtime.Object{
	//		NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//		)},
	//	//newJobFinished(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), ops.ActionDelete, false)...,
	//	),
	//	Key: testNS + "/" + sourceName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeWarning, "InternalError", testJobFailureMessage),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewPullSubscription(sourceName, testNS,
	//			WithPullSubscriptionUID(sourceUID),
	//			WithPullSubscriptionObjectMetaGeneration(generation),
	//			WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
	//				Project: testProject,
	//				Topic:   testTopicID,
	//				Secret:  &secret,
	//			}),
	//			WithPullSubscriptionSink(sinkGVK, sinkName),
	//			WithPullSubscriptionReady(sinkURI),
	//			WithPullSubscriptionDeleted,
	//			WithPullSubscriptionSubscription(testSubscriptionID),
	//			WithPullSubscriptionFinalizers(finalizerName),
	//			// updates
	//			WithPullSubscriptionStatusObservedGeneration(generation),
	//			WithPullSubscriptionJobFailure(testSubscriptionID, "DeleteFailed", fmt.Sprintf("Failed to delete Subscription: %q", testJobFailureMessage)),
	//		),
	//	}},
	//	WantErr: true,
	//},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		pubsubBase := &pubsub.PubSubBase{
			Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
		}
		return &Reconciler{
			PubSubBase:             pubsubBase,
			deploymentLister:       listers.GetDeploymentLister(),
			pullSubscriptionLister: listers.GetPullSubscriptionLister(),
			uriResolver:            resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			receiveAdapterImage:    testImage,
			createClientFn:         gpubsub.TestClientCreator(testData["ps"]),
		}
	}))
}

func newReceiveAdapter(ctx context.Context, image string) runtime.Object {
	source := NewPullSubscription(sourceName, testNS,
		WithPullSubscriptionUID(sourceUID),
		WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
			Project: testProject,
			Topic:   testTopicID,
			Secret:  &secret,
		}))
	args := &resources.ReceiveAdapterArgs{
		Image:          image,
		Source:         source,
		Labels:         resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID: testSubscriptionID,
		SinkURI:        sinkURI,
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

//func newJob(owner kmeta.OwnerRefable, action string) runtime.Object {
//	days7 := 7 * 24 * time.Hour
//	secs30 := 30 * time.Second
//	return operations.NewSubscriptionOps(operations.SubArgs{
//		Image:               testImage,
//		Action:              action,
//		ProjectID:           testProject,
//		TopicID:             testTopicID,
//		SubscriptionID:      testSubscriptionID,
//		AckDeadline:         secs30,
//		RetainAckedMessages: false,
//		RetentionDuration:   days7,
//		Secret:              secret,
//		Owner:               owner,
//	})
//}

//func newJobFinished(owner kmeta.OwnerRefable, action string, success bool) []runtime.Object {
//	days7 := 7 * 24 * time.Hour
//	secs30 := 30 * time.Second
//	job := operations.NewSubscriptionOps(operations.SubArgs{
//		Image:               testImage,
//		Action:              action,
//		ProjectID:           testProject,
//		TopicID:             testTopicID,
//		SubscriptionID:      testSubscriptionID,
//		AckDeadline:         secs30,
//		RetainAckedMessages: false,
//		RetentionDuration:   days7,
//		Owner:               owner,
//	})
//
//	if success {
//		job.Status.Active = 0
//		job.Status.Succeeded = 1
//		job.Status.Conditions = []batchv1.JobCondition{{
//			Type:   batchv1.JobComplete,
//			Status: corev1.ConditionTrue,
//		}, {
//			Type:   batchv1.JobFailed,
//			Status: corev1.ConditionFalse,
//		}}
//	} else {
//		job.Status.Active = 0
//		job.Status.Succeeded = 0
//		job.Status.Conditions = []batchv1.JobCondition{{
//			Type:   batchv1.JobComplete,
//			Status: corev1.ConditionTrue,
//		}, {
//			Type:   batchv1.JobFailed,
//			Status: corev1.ConditionTrue,
//		}}
//	}
//
//	podTerminationMessage := fmt.Sprintf(`{"projectId":"%s"}`, testProject)
//	if !success {
//		podTerminationMessage = fmt.Sprintf(`{"projectId":"%s","reason":"%s"}`, testProject, testJobFailureMessage)
//	}
//
//	jobPod := &corev1.Pod{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "pubsub-s-source-pullsubscription-create-pod",
//			Namespace: testNS,
//			Labels:    map[string]string{"job-name": job.Name},
//		},
//		Status: corev1.PodStatus{
//			ContainerStatuses: []corev1.ContainerStatus{
//				{
//					Name:  "job",
//					Ready: false,
//					State: corev1.ContainerState{
//						Terminated: &corev1.ContainerStateTerminated{
//							ExitCode: 1,
//							Message:  podTerminationMessage,
//						},
//					},
//				},
//			},
//		},
//	}
//
//	return []runtime.Object{job, jobPod}
//}

func TestFinalizers(t *testing.T) {
	testCases := []struct {
		name     string
		original sets.String
		add      bool
		want     sets.String
	}{
		{
			name:     "empty, add",
			original: sets.NewString(),
			add:      true,
			want:     sets.NewString(finalizerName),
		}, {
			name:     "empty, delete",
			original: sets.NewString(),
			add:      false,
			want:     sets.NewString(),
		}, {
			name:     "existing, delete",
			original: sets.NewString(finalizerName),
			add:      false,
			want:     sets.NewString(),
		}, {
			name:     "existing, add",
			original: sets.NewString(finalizerName),
			add:      true,
			want:     sets.NewString(finalizerName),
		}, {
			name:     "existing two, delete",
			original: sets.NewString(finalizerName, "someother"),
			add:      false,
			want:     sets.NewString("someother"),
		}, {
			name:     "existing two, no change",
			original: sets.NewString(finalizerName, "someother"),
			add:      true,
			want:     sets.NewString(finalizerName, "someother"),
		},
	}

	for _, tc := range testCases {
		original := &pubsubv1alpha1.PullSubscription{}
		original.Finalizers = tc.original.List()
		if tc.add {
			addFinalizer(original)
		} else {
			removeFinalizer(original)
		}
		has := sets.NewString(original.Finalizers...)
		diff := has.Difference(tc.want)
		if diff.Len() > 0 {
			t.Errorf("%q failed, diff: %+v", tc.name, diff)
		}
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
