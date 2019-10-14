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
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/channel/resources"

	. "knative.dev/pkg/reconciler/testing"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	channelName = "chan"
	//sinkName    = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	channelUID = channelName + "-abc-123"

	testProject = "test-project-id"
	testTopicID = "cre-chan-" + channelUID

	subscriptionUID        = subscriptionName + "-abc-123"
	subscriptionName       = "testsubscription"
	subscriptionGeneration = 1
)

var (
	topicDNS = channelName + ".mynamespace.svc.cluster.local"
	topicURI = "http://" + topicDNS + "/"

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "Sink",
	}

	subscriberDNS = "subscriber.mynamespace.svc.cluster.local"
	subscriberURI = "http://" + subscriberDNS

	replyDNS = "reply.mynamespace.svc.cluster.local"
	replyURI = "http://" + replyDNS
)

func init() {
	// Add types to scheme
	_ = pubsubv1alpha1.AddToScheme(scheme.Scheme)
}

//
//func newSink() *unstructured.Unstructured {
//	return &unstructured.Unstructured{
//		Object: map[string]interface{}{
//			"apiVersion": "testing.cloud.google.com/v1alpha1",
//			"kind":       "Sink",
//			"metadata": map[string]interface{}{
//				"namespace": testNS,
//				"name":      sinkName,
//			},
//			"status": map[string]interface{}{
//				"address": map[string]interface{}{
//					"hostname": sinkDNS,
//				},
//			},
//		},
//	}
//}

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
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
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
				WithChannelMarkTopicCreating(testTopicID),
				WithChannelSubscribersStatus([]eventingduck.SubscriberStatus(nil)),
			),
		}},
		WantCreates: []runtime.Object{
			newTopic(),
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
				WithChannelTopic(testTopicID),
			),
			newReadyTopic(),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
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
	}, {
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
			Eventf(corev1.EventTypeNormal, "CreatedSubscriber", "Created Subscriber %q", "cre-sub-testsubscription-abc-123"),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
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
			newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: "http://wrong/", ReplyURI: "http://wrong/"}),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "UpdatedSubscriber", "Updated Subscriber %q", "cre-sub-testsubscription-abc-123"),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
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
			Eventf(corev1.EventTypeNormal, "CreatedSubscriber", "Created Subscriber %q", "cre-sub-testsubscription-abc-123"),
			//Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
		},
		WantCreates: []runtime.Object{
			newPullSubscription(eventingduck.SubscriberSpec{UID: subscriptionUID, SubscriberURI: subscriberURI, ReplyURI: replyURI}),
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
			Eventf(corev1.EventTypeNormal, "DeletedSubscriber", "Deleted Subscriber %q", "cre-sub-testsubscription-abc-123"),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
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
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			channelLister:      listers.GetChannelLister(),
			topicLister:        listers.GetTopicLister(),
			subscriptionLister: listers.GetPullSubscriptionLister(),
			tracker:            &MockTracker{},
		}
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
	topic.Status.MarkDeployed()
	topic.Status.MarkTopicReady()
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
