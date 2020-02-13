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

package topic

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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/topic/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	topicName = "hubbub"
	sinkName  = "sink"

	testNS       = "testnamespace"
	testImage    = "test_image"
	topicUID     = topicName + "-abc-123"
	testProject  = "test-project-id"
	testTopicID  = "cloud-run-topic-" + testNS + "-" + topicName + "-" + topicUID
	testTopicURI = "http://" + topicName + "-topic." + testNS + ".svc.cluster.local"

	secretName = "testing-secret"

	failedToReconcileTopicMsg = `Failed to reconcile Pub/Sub topic`
	failedToDeleteTopicMsg    = `Failed to delete Pub/Sub topic`
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
			Name: secretName,
		},
		Key: "testing-key",
	}
)

func init() {
	// Add types to scheme
	_ = pubsubv1alpha1.AddToScheme(scheme.Scheme)
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
		Name: "create client fails",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		OtherTestData: map[string]interface{}{
			"topic": gpubsub.TestClientData{
				CreateClientErr: errors.New("create-client-induced-error"),
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantErr: true,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, finalizerName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicProjectID(testProject),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				WithInitTopicConditions,
				WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileTopicMsg, "create-client-induced-error"))),
		}},
	}, {
		Name: "verify topic exists fails",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		OtherTestData: map[string]interface{}{
			"topic": gpubsub.TestClientData{
				TopicData: gpubsub.TestTopicData{
					ExistsErr: errors.New("topic-exists-induced-error"),
				},
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, "InternalError", "topic-exists-induced-error"),
		},
		WantErr: true,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, finalizerName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicProjectID(testProject),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				WithInitTopicConditions,
				WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileTopicMsg, "topic-exists-induced-error"))),
		}},
	}, {
		Name: "topic does not exist and propagation policy is NoCreateNoDelete",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q does not exist and the topic policy doesn't allow creation", testTopicID),
		},
		WantErr: true,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, finalizerName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicProjectID(testProject),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				WithInitTopicConditions,
				WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: Topic %q does not exist and the topic policy doesn't allow creation", failedToReconcileTopicMsg, testTopicID))),
		}},
	}, {
		Name: "create topic fails",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateNoDelete"),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, "InternalError", "create-topic-induced-error"),
		},
		OtherTestData: map[string]interface{}{
			"topic": gpubsub.TestClientData{
				CreateTopicErr: errors.New("create-topic-induced-error"),
			},
		},
		WantErr: true,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, finalizerName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicProjectID(testProject),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				WithInitTopicConditions,
				WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileTopicMsg, "create-topic-induced-error"))),
		}},
	}, {
		Name: "publisher has not yet been reconciled",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicFinalizers(finalizerName),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateNoDelete"),
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantCreates: []runtime.Object{
			newPublisher(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicProjectID(testProject),
				WithTopicFinalizers(finalizerName),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				WithInitTopicConditions,
				WithTopicReady(testTopicID),
				WithTopicPublisherNotConfigured()),
		}},
	},
		{
			Name: "the status of publisher is false",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
				),
				newSink(),
				newSecret(),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				ProvideResource("create", "services", makeFalseStatusPublisher("PublisherNotDeployed", "PublisherNotDeployed")),
			},
			WantCreates: []runtime.Object{
				newPublisher(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicProjectID(testProject),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					// Updates
					WithInitTopicConditions,
					WithTopicReady(testTopicID),
					WithTopicPublisherNotDeployed("PublisherNotDeployed", "PublisherNotDeployed")),
			}},
		}, {
			Name: "the status of publisher is unknown",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
				),
				newSink(),
				newSecret(),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				ProvideResource("create", "services", makeUnknownStatusPublisher("PublisherUnknown", "PublisherUnknown")),
			},
			WantCreates: []runtime.Object{
				newPublisher(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicProjectID(testProject),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					// Updates
					WithInitTopicConditions,
					WithTopicReady(testTopicID),
					WithTopicPublisherUnknown("PublisherUnknown", "PublisherUnknown")),
			}},
		}, {
			Name: "topic successfully reconciles and is ready, with retry",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
				),
				newSink(),
				newSecret(),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ReadinessChanged", "Topic %q became ready", topicName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				ProvideResource("create", "services", makeReadyPublisher()),
				func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					if attempts != 0 || !action.Matches("update", "topics") {
						return false, nil, nil
					}
					attempts++
					return true, nil, apierrs.NewConflict(v1alpha1.Resource("foo"), "bar", errors.New("foo"))
				},
			},
			WantCreates: []runtime.Object{
				newPublisher(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicProjectID(testProject),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					// Updates
					WithInitTopicConditions,
					WithTopicReady(testTopicID),
					WithTopicPublisherDeployed,
					WithTopicAddress(testTopicURI)),
			}, {
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicProjectID(testProject),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					// Updates
					WithInitTopicConditions,
					WithTopicReady(testTopicID),
					WithTopicPublisherDeployed,
					WithTopicAddress(testTopicURI)),
			}},
		}, {
			Name: "topic successfully reconciles and reuses existing publisher",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
				),
				newSink(),
				newSecret(),
				makeReadyPublisher(),
				NewService(topicName+"-topic", testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.GetLabels(controllerAgentName, topicName)),
					WithServicePorts(servicePorts())),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ReadinessChanged", "Topic %q became ready", topicName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
			},
			WithReactors: []clientgotesting.ReactionFunc{},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicProjectID(testProject),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					// Updates
					WithInitTopicConditions,
					WithTopicReady(testTopicID),
					WithTopicPublisherDeployed,
					WithTopicAddress(testTopicURI)),
			}},
		}, {
			Name: "delete topic - policy CreateNoDelete",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					WithTopicDeleted,
				),
				newSink(),
				newSecret(),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, topicName, ""),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateNoDelete"),
					WithInitTopicConditions,
					WithTopicDeleted),
			}},
		}, {
			Name: "delete topic - policy CreateDelete",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateDelete"),
					WithTopicTopicID(topicName),
					WithTopicDeleted,
				),
				newSink(),
				newSecret(),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, topicName, ""),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateDelete"),
					WithInitTopicConditions,
					WithTopicNoTopic("TopicDeleted", fmt.Sprintf("Successfully deleted Pub/Sub topic: %s", topicName)),
					WithTopicDeleted),
			}},
		}, {
			Name: "fail to delete - policy CreateDelete",
			Objects: []runtime.Object{
				NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateDelete"),
					WithTopicTopicID(topicName),
					WithTopicDeleted,
				),
				newSink(),
				newSecret(),
			},
			Key: testNS + "/" + topicName,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "delete-topic-induced-error"),
			},
			OtherTestData: map[string]interface{}{
				"topic": gpubsub.TestClientData{
					TopicData: gpubsub.TestTopicData{
						Exists:    true,
						DeleteErr: errors.New("delete-topic-induced-error"),
					},
				},
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTopic(topicName, testNS,
					WithTopicUID(topicUID),
					WithTopicFinalizers(finalizerName),
					WithTopicSpec(pubsubv1alpha1.TopicSpec{
						Project: testProject,
						Topic:   testTopicID,
						Secret:  &secret,
					}),
					WithTopicPropagationPolicy("CreateDelete"),
					WithInitTopicConditions,
					WithTopicTopicID(topicName),
					WithTopicNoTopic("TopicDeleteFailed", fmt.Sprintf("%s: %s", failedToDeleteTopicMsg, "delete-topic-induced-error")),
					WithTopicDeleted),
			}},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		pubsubBase := &pubsub.PubSubBase{
			Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
		}
		return &Reconciler{
			PubSubBase:     pubsubBase,
			topicLister:    listers.GetTopicLister(),
			serviceLister:  listers.GetV1ServiceLister(),
			publisherImage: testImage,
			createClientFn: gpubsub.TestClientCreator(testData["topic"]),
		}
	}))

}

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
		original := &pubsubv1alpha1.Topic{}
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

func ProvideResource(verb, resource string, obj runtime.Object) clientgotesting.ReactionFunc {
	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if !action.Matches(verb, resource) {
			return false, nil, nil
		}
		return true, obj, nil
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

func ownerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         "pubsub.cloud.google.com/v1alpha1",
		Kind:               "Topic",
		Name:               topicName,
		UID:                topicUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}}
}

func servicePorts() []corev1.ServicePort {
	svcPorts := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(8080),
		}, {
			Name: "metrics",
			Port: 9090,
		},
	}
	return svcPorts
}

func makeReadyPublisher() *servingv1.Service {
	pub := newPublisher()
	pub.Status.Conditions = []apis.Condition{{
		Type:   apis.ConditionReady,
		Status: "True",
	}}
	uri, _ := apis.ParseURL(testTopicURI)
	pub.Status.Address = &duckv1.Addressable{
		URL: uri,
	}
	return pub
}

func makeUnknownStatusPublisher(reason, message string) *servingv1.Service {
	pub := newPublisher()
	pub.Status.Conditions = []apis.Condition{{
		Type:    apis.ConditionReady,
		Status:  "Unknown",
		Reason:  reason,
		Message: message,
	}}
	return pub
}

func makeFalseStatusPublisher(reason, message string) *servingv1.Service {
	pub := newPublisher()
	pub.Status.Conditions = []apis.Condition{{
		Type:    apis.ConditionReady,
		Status:  "False",
		Reason:  reason,
		Message: message,
	}}
	return pub
}

func newPublisher() *servingv1.Service {
	topic := NewTopic(topicName, testNS,
		WithTopicUID(topicUID),
		WithTopicSpec(pubsubv1alpha1.TopicSpec{
			Project: testProject,
			Topic:   testTopicID,
			Secret:  &secret,
		}))
	args := &resources.PublisherArgs{
		Image:  testImage,
		Topic:  topic,
		Labels: resources.GetLabels(controllerAgentName, topicName),
	}
	return resources.MakePublisher(args)
}
