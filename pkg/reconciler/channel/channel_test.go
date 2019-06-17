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
	"github.com/knative/pkg/kmeta"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	"github.com/knative/pkg/tracker"

	pubsubv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/operations"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"

	. "github.com/knative/pkg/reconciler/testing"

	. "github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/testing"
)

const (
	channelName = "pschan"
	sinkName    = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	channelUID = channelName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = channelUID + "-TOPIC"
	testSubscriptionID = "cloud-run-events-" + testNS + "-" + channelName + "-" + channelUID
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
				WithChannelSpec(pubsubv1alpha1.ChannelSpec{
					Project:            testProject,
					ServiceAccountName: testServiceAccount,
				}),
			),
			newSink(),
		},
		Key: testNS + "/" + channelName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q finalizers", channelName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Channel %q", channelName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSpec(pubsubv1alpha1.ChannelSpec{
					Project:            testProject,
					ServiceAccountName: testServiceAccount,
				}),
				// Updates
				WithInitChannelConditions,
				WithChannelMarkTopicCreating(sinkURI),
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewChannel(channelName, testNS, WithChannelUID(channelUID)), operations.ActionCreate),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, true),
		},
	},
	//{
	//	Name: "successful create",
	//	Objects: []runtime.Object{
	//		NewChannel(channelName, testNS,
	//			WithChannelUID(channelUID),
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelSink(sinkGVK, sinkName),
	//			WithChannelSubscription(testSubscriptionID),
	//		),
	//		newSink(),
	//	},
	//	Key: testNS + "/" + channelName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", `Updated Channel "source"`),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewChannel(channelName, testNS,
	//			WithChannelUID(channelUID),
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelSink(sinkGVK, sinkName),
	//			WithChannelSubscription(testSubscriptionID),
	//			// Updates
	//			WithInitChannelConditions,
	//			WithChannelReady(sinkURI),
	//		),
	//	}},
	//	WantCreates: []runtime.Object{
	//		newReceiveAdapter(),
	//	},
	//}, {
	//	Name: "successful create - reuse existing receive adapter",
	//	Objects: []runtime.Object{
	//		NewChannel(channelName, testNS,
	//			WithChannelUID(channelUID),
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelSink(sinkGVK, sinkName),
	//			WithChannelSubscription(testSubscriptionID),
	//		),
	//		newSink(),
	//		newReceiveAdapter(),
	//	},
	//	Key: testNS + "/" + channelName,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeNormal, "Updated", `Updated Channel "source"`),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewChannel(channelName, testNS,
	//			WithChannelUID(channelUID),
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelSink(sinkGVK, sinkName),
	//			WithChannelSubscription(testSubscriptionID),
	//			// Updates
	//			WithInitChannelConditions,
	//			WithChannelReady(sinkURI),
	//		),
	//	}},
	//}, {
	//	Name: "cannot get sink",
	//	Objects: []runtime.Object{
	//		NewChannel(channelName, testNS,
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelSink(sinkGVK, sinkName),
	//		)},
	//	Key:     testNS + "/" + channelName,
	//	WantErr: true,
	//	WantEvents: []string{
	//		Eventf(corev1.EventTypeWarning, "InternalError", `sinks.testing.cloud.run "sink" not found`),
	//	},
	//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
	//		Object: NewChannel(channelName, testNS,
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelSink(sinkGVK, sinkName),
	//			// updates
	//			WithInitChannelConditions,
	//			WithChannelSinkNotFound(),
	//		),
	//	}},
	//},
	//{
	//	Name: "deleting - delete subscription",
	//	Objects: []runtime.Object{
	//		NewChannel(channelName, testNS,
	//			WithChannelUID(channelUID),
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelReady(sinkURI),
	//			WithChannelFinalizers(finalizerName),
	//			WithChannelDeleted,
	//		),
	//	},
	//	Key: testNS + "/" + channelName,
	//WantEvents: []string{
	//	Eventf(corev1.EventTypeNormal, "Updated", `Updated Channel "source" finalizers`),
	//	Eventf(corev1.EventTypeNormal, "Updated", `Updated Channel "source"`),
	//},
	//WantPatches: []clientgotesting.PatchActionImpl{
	//	patchFinalizers(testNS, channelName, false),
	//},
	//},
	//{
	//	Name: "deleting final stage",
	//	Objects: []runtime.Object{
	//		NewChannel(channelName, testNS,
	//			WithChannelUID(channelUID),
	//			WithChannelSpec(pubsubv1alpha1.ChannelSpec{
	//				Project:            testProject,
	//				Topic:              testTopicID,
	//				ServiceAccountName: testServiceAccount,
	//			}),
	//			WithChannelReady(sinkURI),
	//			WithChannelDeleted,
	//		),
	//	},
	//	Key: testNS + "/" + channelName,
	//},

	// TODO:
	//			Name: "successful create event types",
	//			Name: "cannot create event types",

	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		pubsubBase := &pubsub.PubSubBase{
			Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
			SubscriptionOpsImage: testImage + "sub",
			TopicOpsImage:        testImage + "top",
		}
		return &Reconciler{
			PubSubBase:       pubsubBase,
			deploymentLister: listers.GetDeploymentLister(),
			channelLister:    listers.GetChannelLister(),
			tracker:          tracker.New(func(string) {}, 0),
			invokerImage:     testImage,
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
		original := &pubsubv1alpha1.Channel{}
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

func patchFinalizers(namespace, name string, add bool) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	var fname string
	if add {
		fname = fmt.Sprintf("%q", finalizerName)
	}
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func newTopicJob(owner kmeta.OwnerRefable, action string) runtime.Object {
	return operations.NewTopicOps(operations.TopicArgs{
		Image:     testImage,
		Action:    action,
		ProjectID: testProject,
		TopicID:   testTopicID,
		Owner:     owner,
	})
}
