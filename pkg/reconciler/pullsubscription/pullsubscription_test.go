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

package pullsubscription

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/tracker"

	pubsubv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/operations"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pullsubscription/resources"

	. "github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	sourceName = "source"
	sinkName   = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	sourceUID = sourceName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = sourceUID + "-TOPIC"
	testSubscriptionID = "cre-pull-" + testNS + "-" + sourceName + "-" + sourceUID
)

var (
	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = "http://" + sinkDNS

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
		Name: "incomplete source - sink ref is nil",
		Objects: []runtime.Object{
			NewPullSubscription(sourceName, testNS),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "UpdateFailed", "Failed to update status for PullSubscription %q: missing field(s): spec.sink, spec.topic", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewPullSubscription(sourceName, testNS,
				WithInitPullSubscriptionConditions,
				WithPullSubscriptionSinkNil(),
			),
		}},
	},
		{
			Name: "create subscription",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithInitPullSubscriptionConditions,
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionMarkSink(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithInitPullSubscriptionConditions,
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionMarkSink(sinkURI),
					// Updates
					WithPullSubscriptionMarkSubscribing(testSubscriptionID),
				),
			}},
			WantCreates: []runtime.Object{
				newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), operations.ActionCreate),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, sourceName, true),
			},
		},
		{
			Name: "successful create",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
				),
				newSink(),
				newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), operations.ActionCreate),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
					// Updates
					WithInitPullSubscriptionConditions,
					WithPullSubscriptionReady(sinkURI),
				),
			}},
			WantCreates: []runtime.Object{
				newReceiveAdapter(testImage),
			},
		}, {
			Name: "successful create - reuse existing receive adapter - match",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
				),
				newSink(),
				newReceiveAdapter(testImage),
				newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), operations.ActionCreate),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
					// Updates
					WithInitPullSubscriptionConditions,
					WithPullSubscriptionReady(sinkURI),
				),
			}},
		}, {
			Name: "successful create - reuse existing receive adapter - mismatch",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
				),
				newSink(),
				newReceiveAdapter("old" + testImage),
				newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), operations.ActionCreate),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Verb:      "update",
					Resource:  receiveAdapterGVR(),
				},
				Object: newReceiveAdapter(testImage),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
					// Updates
					WithInitPullSubscriptionConditions,
					WithPullSubscriptionReady(sinkURI),
				),
			}},
		}, {
			Name: "cannot get sink",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `sinks.testing.cloud.run "sink" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					// updates
					WithInitPullSubscriptionConditions,
					WithPullSubscriptionSinkNotFound(),
				),
			}},
		},
		{
			Name: "deleting - delete subscription",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
					WithPullSubscriptionFinalizers(finalizerName),
					WithPullSubscriptionDeleted,
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionSubscription(testSubscriptionID),
					WithPullSubscriptionFinalizers(finalizerName),
					WithPullSubscriptionDeleted,
					// updates
					WithPullSubscriptionMarkUnsubscribing(testSubscriptionID),
				),
			}},
			WantCreates: []runtime.Object{
				newJob(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), operations.ActionDelete),
			},
		},
		{
			Name: "deleting final stage",
			Objects: []runtime.Object{
				NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionDeleted,
					WithPullSubscriptionSubscription(testSubscriptionID),
					WithPullSubscriptionFinalizers(finalizerName),
				),
				newJobFinished(NewPullSubscription(sourceName, testNS, WithPullSubscriptionUID(sourceUID)), operations.ActionDelete, true),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q finalizers", sourceName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated PullSubscription %q", sourceName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewPullSubscription(sourceName, testNS,
					WithPullSubscriptionUID(sourceUID),
					WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
						Project: testProject,
						Topic:   testTopicID,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionDeleted,
					WithPullSubscriptionSubscription(testSubscriptionID),
					WithPullSubscriptionFinalizers(finalizerName),
					// updates
					WithPullSubscriptionMarkNoSubscription(testSubscriptionID),
				),
			}},
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
		pubsubBase := &pubsub.PubSubBase{
			Base:                 reconciler.NewBase(ctx, controllerAgentName, cmw),
			SubscriptionOpsImage: testImage,
		}
		return &Reconciler{
			PubSubBase:          pubsubBase,
			deploymentLister:    listers.GetDeploymentLister(),
			sourceLister:        listers.GetPullSubscriptionLister(),
			tracker:             tracker.New(func(string) {}, 0),
			receiveAdapterImage: testImage,
		}
	}))

}

func newReceiveAdapter(image string) runtime.Object {
	source := NewPullSubscription(sourceName, testNS,
		WithPullSubscriptionUID(sourceUID),
		WithPullSubscriptionSpec(pubsubv1alpha1.PullSubscriptionSpec{
			Project: testProject,
			Topic:   testTopicID,
		}))
	args := &resources.ReceiveAdapterArgs{
		Image:          image,
		Source:         source,
		Labels:         resources.GetLabels(controllerAgentName, sourceName),
		SubscriptionID: testSubscriptionID,
		SinkURI:        sinkURI,
	}
	return resources.MakeReceiveAdapter(args)
}

func receiveAdapterGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployment",
	}
}

func newJob(owner kmeta.OwnerRefable, action string) runtime.Object {
	days7 := 7 * 24 * time.Hour
	secs30 := 30 * time.Second
	return operations.NewSubscriptionOps(operations.SubArgs{
		Image:               testImage,
		Action:              action,
		ProjectID:           testProject,
		TopicID:             testTopicID,
		SubscriptionID:      testSubscriptionID,
		AckDeadline:         secs30,
		RetainAckedMessages: false,
		RetentionDuration:   days7,
		Owner:               owner,
	})
}

func newJobFinished(owner kmeta.OwnerRefable, action string, success bool) runtime.Object {
	days7 := 7 * 24 * time.Hour
	secs30 := 30 * time.Second
	job := operations.NewSubscriptionOps(operations.SubArgs{
		Image:               testImage,
		Action:              action,
		ProjectID:           testProject,
		TopicID:             testTopicID,
		SubscriptionID:      testSubscriptionID,
		AckDeadline:         secs30,
		RetainAckedMessages: false,
		RetentionDuration:   days7,
		Owner:               owner,
	})

	if success {
		job.Status.Active = 0
		job.Status.Succeeded = 1
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}, {
			Type:   batchv1.JobFailed,
			Status: corev1.ConditionFalse,
		}}
	} else {
		job.Status.Active = 0
		job.Status.Succeeded = 0
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}, {
			Type:   batchv1.JobFailed,
			Status: corev1.ConditionTrue,
		}}
	}

	return job
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
