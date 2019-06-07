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

package pubsubsource

import (
	"context"
	"errors"
	"fmt"
	"github.com/knative/pkg/configmap"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	eventsv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsubutil/fakepubsub"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsubsource/resources"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	"github.com/knative/pkg/tracker"

	. "github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"
)

const (
	sourceName = "source"
	sinkName   = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	sourceUID = sourceName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = sourceUID + "-TOPIC"
	testSubscriptionID = "test-subscription-id"
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
	_ = eventsv1alpha1.AddToScheme(scheme.Scheme)
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

func TestAllCases_pubsub_clientfail(t *testing.T) {
	creator := fakepubsub.Creator(fakepubsub.CreatorData{
		ClientCreateErr: errors.New("test-induced-error"),
	})

	table := TableTest{{
		Name: "cannot create client",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
				WithPubSubSourceReady(sinkURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "test-induced-error"),
		},
	}, {
		Name: "deleting - cannot create client",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceReady(sinkDNS),
				WithPubSubSourceDeleted,
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "test-induced-error"),
		},
	},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
			deploymentLister:    listers.GetDeploymentLister(),
			sourceLister:        listers.GetPubSubSourceLister(),
			tracker:             tracker.New(func(string) {}, 0),
			receiveAdapterImage: "img",
			pubSubClientCreator: creator,
		}
	}))
}

func TestAllCases_pubsub_subgetfail(t *testing.T) {
	creator := fakepubsub.Creator(fakepubsub.CreatorData{
		ClientData: fakepubsub.ClientData{
			SubscriptionData: fakepubsub.SubscriptionData{
				ExistsErr: errors.New("test-induced-error"),
			},
		},
	})

	table := TableTest{{
		Name: "error checking subscription exists",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
				WithPubSubSourceReady(sinkURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "test-induced-error"),
		},
	}, {
		Name: "deleting - error checking subscription exists",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceReady(sinkDNS),
				WithPubSubSourceDeleted,
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "test-induced-error"),
		},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
			deploymentLister:    listers.GetDeploymentLister(),
			sourceLister:        listers.GetPubSubSourceLister(),
			tracker:             tracker.New(func(string) {}, 0),
			receiveAdapterImage: "img",
			pubSubClientCreator: creator,
		}
	}))
}

func TestAllCases_pubsub_sub_delete_fail(t *testing.T) {
	creator := fakepubsub.Creator(fakepubsub.CreatorData{
		ClientData: fakepubsub.ClientData{
			SubscriptionData: fakepubsub.SubscriptionData{
				Exists:    true,
				DeleteErr: errors.New("delete-test-induced-error"),
			},
		},
	})

	table := TableTest{{
		Name: "deleting - cannot delete subscription",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceReady(sinkDNS),
				WithPubSubSourceDeleted,
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "delete-test-induced-error"),
		},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
			deploymentLister:    listers.GetDeploymentLister(),
			sourceLister:        listers.GetPubSubSourceLister(),
			tracker:             tracker.New(func(string) {}, 0),
			receiveAdapterImage: "img",
			pubSubClientCreator: creator,
		}
	}))
}

func TestAllCases_pubsub_sub_create_fail(t *testing.T) {
	creator := fakepubsub.Creator(fakepubsub.CreatorData{
		ClientData: fakepubsub.ClientData{
			CreateSubErr: errors.New("create-test-induced-error"),
		},
	})

	table := TableTest{{
		Name: "cannot create subscription",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
				WithPubSubSourceReady(sinkURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-test-induced-error"),
		},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
			deploymentLister:    listers.GetDeploymentLister(),
			sourceLister:        listers.GetPubSubSourceLister(),
			tracker:             tracker.New(func(string) {}, 0),
			receiveAdapterImage: "img",
			pubSubClientCreator: creator,
		}
	}))
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
		Name: "incomplete source - sink ref is nil",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "sink ref is nil"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubSource(sourceName, testNS,
				WithInitPubSubSourceConditions,
				WithPubSubSourceSinkNotFound(),
			),
		}},
	}, {
		Name: "successful create",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
			),
			newSink(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", `Updated PubSubSource "source" finalizers`),
			Eventf(corev1.EventTypeNormal, "Updated", `Updated PubSubSource "source"`),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
				// Updates
				WithInitPubSubSourceConditions,
				WithPubSubSourceReady(sinkURI),
			),
		}},
		WantCreates: []runtime.Object{
			newReceiveAdapter(),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
	}, {
		Name: "successful create - reuse existing receive adapter",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
			),
			newSink(),
			newReceiveAdapter(),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", `Updated PubSubSource "source" finalizers`),
			Eventf(corev1.EventTypeNormal, "Updated", `Updated PubSubSource "source"`),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
				// Updates
				WithInitPubSubSourceConditions,
				WithPubSubSourceReady(sinkURI),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
	}, {
		Name: "cannot get sink",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
			)},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `sinks.testing.cloud.run "sink" not found`),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPubSubSource(sourceName, testNS,
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceSink(sinkGVK, sinkName),
				// updates
				WithInitPubSubSourceConditions,
				WithPubSubSourceSinkNotFound(),
			),
		}},
	}, {
		Name: "deleting - remove finalizer",
		Objects: []runtime.Object{
			NewPubSubSource(sourceName, testNS,
				WithPubSubSourceUID(sourceUID),
				WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
					GoogleCloudProject: testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithPubSubSourceReady(sinkURI),
				WithPubSubSourceFinalizers(finalizerName),
				WithPubSubSourceDeleted,
			),
		},
		Key: testNS + "/" + sourceName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", `Updated PubSubSource "source" finalizers`),
			//Eventf(corev1.EventTypeNormal, "Updated", `Updated PubSubSource "source"`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, false),
		},
	},

	// TODO:
	//			Name: "successful create event types",
	//			Name: "cannot create event types",

	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
			deploymentLister:    listers.GetDeploymentLister(),
			sourceLister:        listers.GetPubSubSourceLister(),
			tracker:             tracker.New(func(string) {}, 0),
			receiveAdapterImage: testImage,
			pubSubClientCreator: fakepubsub.Creator(fakepubsub.CreatorData{}),
		}
	}))

}

func newReceiveAdapter() runtime.Object {
	source := NewPubSubSource(sourceName, testNS,
		WithPubSubSourceUID(sourceUID),
		WithPubSubSourceSpec(eventsv1alpha1.PubSubSourceSpec{
			GoogleCloudProject: testProject,
			Topic:              testTopicID,
			ServiceAccountName: testServiceAccount,
		}))
	args := &resources.ReceiveAdapterArgs{
		Image:          testImage,
		Source:         source,
		Labels:         resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID: testSubscriptionID,
		SinkURI:        sinkURI,
	}
	return resources.MakeReceiveAdapter(args)
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
		original := &eventsv1alpha1.PubSubSource{}
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
