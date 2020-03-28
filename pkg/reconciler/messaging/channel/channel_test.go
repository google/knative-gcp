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

package channel

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/messaging/v1alpha1/channel"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/messaging/channel/resources"
	. "knative.dev/pkg/reconciler/testing"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	channelName = "chan"

	testNS = "testnamespace"

	channelUID = channelName + "-abc-123"

	testProject   = "test-project-id"
	testTopicID   = "cre-chan-" + channelUID
	testTopicName = "cre-chan-" + channelName

	subscriptionUID  = subscriptionName + "-abc-123"
	subscriptionName = "testsubscription"
)

var (
	trueVal  = true
	falseVal = false

	topicDNS = channelName + ".mynamespace.svc.cluster.local"
	topicURI = "http://" + topicDNS + "/"

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "Sink",
	}

	subscriberDNS = "subscriber.mynamespace.svc.cluster.local"
	subscriberURI = apis.HTTP(subscriberDNS)

	replyDNS = "reply.mynamespace.svc.cluster.local"
	replyURI = apis.HTTP(replyDNS)

	gServiceAccount = "test123@test123.iam.gserviceaccount.com"
)

func init() {
	// Add types to scheme
	_ = pubsubv1alpha1.AddToScheme(scheme.Scheme)
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

// TODO add a unit test for successfully creating a k8s service account, after issue https://github.com/google/knative-gcp/issues/657 gets solved.
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
		Name: "create topic",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithChannelDefaults,
			),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
			Eventf(corev1.EventTypeNormal, "TopicCreated", "Created Topic %q", testTopicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithChannelDefaults,
				// Updates
				WithInitChannelConditions,
				WithChannelSubscribersStatus([]eventingduck.SubscriberStatus(nil)),
				WithChannelTopicID(testTopicID),
				WithChannelTopicUnknown("TopicNotConfigured", "Topic has not yet been reconciled"),
			),
		}},
		WantCreates: []runtime.Object{
			newTopic(),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, true),
		},
	}, {
		Name: "topic ready",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithInitChannelConditions,
				WithChannelDefaults,
			),
			newReadyTopic(),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithInitChannelConditions,
				WithChannelDefaults,
				WithChannelTopic(testTopicID),
				// Updates
				WithChannelAddress(topicURI),
				WithChannelSubscribersStatus([]eventingduck.SubscriberStatus(nil)),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, true),
		},
	}, {
		Name: "the status of topic is false",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithInitChannelConditions,
				WithChannelDefaults,
			),
			newFalseTopic(),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithInitChannelConditions,
				WithChannelDefaults,
				WithChannelTopic(testTopicID),
				// Updates
				WithChannelAddress(topicURI),
				WithChannelSubscribersStatus([]eventingduck.SubscriberStatus(nil)),
				WithChannelTopicFailed("PublisherStatus", "Publisher has no Ready type status"),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, true),
		},
	},
		{
			Name: "new subscriber",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
				),
				newReadyTopic(),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
				Eventf(corev1.EventTypeNormal, "SubscriberCreated", "Created Subscriber %q", "cre-sub-testsubscription-abc-123"),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					// Updates
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{
						{UID: subscriptionUID, Ready: corev1.ConditionFalse, Message: "PullSubscription cre-sub-testsubscription-abc-123 is not ready"},
					}),
				),
			}},
			WantCreates: []runtime.Object{
				newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, channelName, true),
			},
		}, {
			Name: "update subscriber",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, Generation: 2, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{
						{UID: subscriptionUID, ObservedGeneration: 1},
					}),
				),
				newReadyTopic(),
				newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: apis.HTTP("wrong"), ReplyURI: apis.HTTP("wrong")}),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
				Eventf(corev1.EventTypeNormal, "SubscriberUpdated", "Updated Subscriber %q", "cre-sub-testsubscription-abc-123"),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, Generation: 2, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					// Updates
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{
						{UID: subscriptionUID, ObservedGeneration: 2, Ready: corev1.ConditionFalse, Message: "PullSubscription cre-sub-testsubscription-abc-123 is not ready"},
					}),
				),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, channelName, true),
			},
		}, {
			Name: "subscriber already exists owned by other channel",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
				),
				newReadyTopic(),
				newPullSubscriptionWithOwner(
					eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					NewChannel("other-channel", testNS, WithChannelUID("other-id"), WithInitChannelConditions),
				),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
				Eventf(corev1.EventTypeWarning, "SubscriberNotOwned", "Subscriber %q is not owned by this channel", "cre-sub-testsubscription-abc-123"),
				Eventf(corev1.EventTypeWarning, reconciledSubscribersFailedReason, "Reconcile Subscribers failed with: channel %q does not own subscriber %q", channelName, "cre-sub-testsubscription-abc-123"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					// Updates
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{}),
				),
			}},
			WantCreates: []runtime.Object{
				newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, channelName, true),
			},
		}, {
			Name: "subscriber already exists in status owned by other channel",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{
						{UID: subscriptionUID, ObservedGeneration: 1},
					}),
				),
				newReadyTopic(),
				newPullSubscriptionWithOwner(
					eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					NewChannel("other-channel", testNS, WithChannelUID("other-id"), WithInitChannelConditions),
				),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
				Eventf(corev1.EventTypeWarning, "SubscriberNotOwned", "Subscriber %q is not owned by this channel", "cre-sub-testsubscription-abc-123"),
				Eventf(corev1.EventTypeWarning, reconciledSubscribersFailedReason, "Reconcile Subscribers failed with: channel %q does not own subscriber %q", channelName, "cre-sub-testsubscription-abc-123"),
			},
			WantCreates: []runtime.Object{
				newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, channelName, true),
			},
		}, {
			Name: "update - subscriber missing",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{
						{UID: subscriptionUID, Generation: 1, SubscriberURI: subscriberURI, ReplyURI: replyURI},
					}),
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{
						{UID: subscriptionUID, ObservedGeneration: 1, Ready: corev1.ConditionFalse, Message: "PullSubscription cre-sub-testsubscription-abc-123 is not ready"},
					}),
				),
				newReadyTopic(),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
				Eventf(corev1.EventTypeNormal, "SubscriberCreated", "Created Subscriber %q", "cre-sub-testsubscription-abc-123"),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
			},
			WantCreates: []runtime.Object{
				newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, channelName, true),
			},
		}, {
			Name: "delete subscriber",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{}),
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{
						{UID: subscriptionUID},
					}),
				),
				newReadyTopic(),
				newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", channelName),
				Eventf(corev1.EventTypeNormal, "SubscriberDeleted", "Deleted Subscriber %q", "cre-sub-testsubscription-abc-123"),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Channel reconciled: "%s/%s"`, testNS, channelName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelDefaults,
					WithChannelTopic(testTopicID),
					WithChannelAddress(topicURI),
					WithChannelSubscribers([]eventingduck.SubscriberSpec{}),
					// Updates
					WithChannelSubscribersStatus([]eventingduck.SubscriberStatus{}),
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: "testnamespace", Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}},
					Name: "cre-sub-testsubscription-abc-123",
				},
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, channelName, true),
			},
		}, {
			Name: "delete channel failed with getting k8s service account error",
			Objects: []runtime.Object{
				NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelGCPServiceAccount(gServiceAccount),
					WithChannelDefaults,
					WithChannelDeletionTimestamp,
					WithChannelServiceAccountName("test123"),
				),
			},
			Key: testNS + "/" + channelName,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "WorkloadIdentityDeleteFailed", `Failed to delete Channel workload identity: getting k8s service account failed with: serviceaccounts "test123" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelSpec(v1alpha1.ChannelSpec{
						Project: testProject,
					}),
					WithInitChannelConditions,
					WithChannelGCPServiceAccount(gServiceAccount),
					WithChannelDefaults,
					WithChannelServiceAccountName("test123"),
					WithChannelDeletionTimestamp,
					WithChannelWorkloadIdentityFailed("WorkloadIdentityDeleteFailed", `serviceaccounts "test123" not found`),
				),
			}},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, _ map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			Base:                   reconciler.NewBase(ctx, controllerAgentName, cmw),
			Identity:               identity.NewIdentity(ctx),
			channelLister:          listers.GetChannelLister(),
			topicLister:            listers.GetTopicLister(),
			pullSubscriptionLister: listers.GetPullSubscriptionLister(),
			serviceAccountLister:   listers.GetServiceAccountLister(),
		}
		return channel.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetChannelLister(), r.Recorder, r)
	}))

}

func newTopic() *pubsubv1alpha1.Topic {
	channel := NewChannel(channelName, testNS,
		WithChannelUID(channelUID),
		WithChannelSpec(v1alpha1.ChannelSpec{
			Project: testProject,
		}),
		WithInitChannelConditions,
		WithChannelTopic(testTopicID),
		WithChannelDefaults)

	return resources.MakeTopic(&resources.TopicArgs{
		Owner:   channel,
		Name:    resources.GeneratePublisherName(channel),
		Project: channel.Spec.Project,
		Topic:   channel.Status.TopicID,
		Secret:  channel.Spec.Secret,
		Labels:  resources.GetLabels(controllerAgentName, channel.Name, string(channel.UID)),
	})
}

func newReadyTopic() *pubsubv1alpha1.Topic {
	topic := newTopic()
	url, _ := apis.ParseURL(topicURI)
	topic.Status.SetAddress(url)
	topic.Status.MarkPublisherDeployed()
	topic.Status.MarkTopicReady()
	return topic
}

func newFalseTopic() *pubsubv1alpha1.Topic {
	topic := newTopic()
	url, _ := apis.ParseURL(topicURI)
	topic.Status.SetAddress(url)
	topic.Status.MarkPublisherNotDeployed("PublisherStatus", "Publisher has no Ready type status")
	return topic
}

func newPullSubscription(subscriber eventingduck.SubscriberSpec) *pubsubv1alpha1.PullSubscription {
	channel := NewChannel(channelName, testNS,
		WithChannelUID(channelUID),
		WithChannelSpec(v1alpha1.ChannelSpec{
			Project: testProject,
		}),
		WithInitChannelConditions,
		WithChannelTopic(testTopicID),
		WithChannelDefaults)

	return newPullSubscriptionWithOwner(subscriber, channel)
}

func newPullSubscriptionWithOwner(subscriber eventingduck.SubscriberSpec, channel *v1alpha1.Channel) *pubsubv1alpha1.PullSubscription {
	return resources.MakePullSubscription(&resources.PullSubscriptionArgs{
		Owner:       channel,
		Name:        resources.GenerateSubscriptionName(subscriber.UID),
		Project:     channel.Spec.Project,
		Topic:       channel.Status.TopicID,
		Secret:      channel.Spec.Secret,
		Labels:      resources.GetPullSubscriptionLabels(controllerAgentName, channel.Name, resources.GenerateSubscriptionName(subscriber.UID), string(channel.UID)),
		Annotations: resources.GetPullSubscriptionAnnotations(channel.Name),
		Subscriber:  subscriber,
	})
}
