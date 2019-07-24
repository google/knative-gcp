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
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/tracker"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/channel/resources"

	. "knative.dev/pkg/reconciler/testing"

	. "github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/testing"
)

const (
	channelName = "chan"
	sinkName    = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	channelUID = channelName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = "cloud-run-chan-" + channelName + "-" + channelUID
	testSubscriptionID = "cloud-run-chan-" + channelName + "-" + channelUID
	testServiceAccount = "test-project-id"
)

var (
	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = "http://" + sinkDNS + "/"

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.run",
		Version: "v1alpha1",
		Kind:    "Sink",
	}
)

func init() {
	// Add types to scheme
	_ = pubsubv1alpha1.AddToScheme(scheme.Scheme)
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
			newSink(),
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
			),
		}},
		WantCreates: []runtime.Object{
			newTopic(),
		},
	}, {
		Name: "successful created topic",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithInitChannelConditions,
				WithChannelTopic(testTopicID),
				WithChannelDefaults,
			),
			newTopic(),
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
				WithChannelTopic(testTopicID),
				// Updates

			),
		}},
		//WantCreates: []runtime.Object{
		//
		//},
	}, /* {
		Name: "successful create - reuse existing receive adapter",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithInitChannelConditions,
				WithChannelTopic(testTopicID),
			),
			//newTopicJob(NewChannel(channelName, testNS, WithChannelUID(channelUID)), operations.ActionCreate),
			//newInvoker(),
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
				// Updates
				WithChannelReady(testTopicID),
			),
		}},
	}, {
		Name: "deleting - delete topic",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithChannelReady(testTopicID),
				WithChannelDeleted,
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
				WithChannelReady(testTopicID),
				WithChannelDeleted,
				// Updates
				WithChannelTopicDeleting(testTopicID),
			),
		}},
		WantCreates: []runtime.Object{
			//newTopicJob(NewChannel(channelName, testNS, WithChannelUID(channelUID)), operations.ActionDelete),
		},
	}, {
		Name: "deleting final stage",
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithChannelReady(testTopicID),
				WithChannelDeleted,
				WithChannelTopicDeleting(testTopicID),
			),
			//newTopicJobFinished(NewChannel(channelName, testNS, WithChannelUID(channelUID)), operations.ActionDelete, true),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q finalizers", channelName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(v1alpha1.ChannelSpec{
					Project: testProject,
				}),
				WithChannelReady(testTopicID),
				WithChannelDeleted,
				// Updates
				WithChannelTopicDeleted(testTopicID),
			),
		}},
	},
	*/
	// TODO: subscriptions.
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			channelLister:      listers.GetChannelLister(),
			topicLister:        listers.GetTopicLister(),
			subscriptionLister: listers.GetPullSubscriptionLister(),
			tracker:            tracker.New(func(string) {}, 0),
		}
	}))

}

func newTopic() runtime.Object {

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
		Project: channel.Spec.Project,
		Topic:   channel.Status.TopicID,
		Secret:  channel.Spec.Secret,
		Labels:  resources.GetLabels(controllerAgentName, channel.Name),
	})
}
