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
	"fmt"
	"github.com/knative/pkg/kmeta"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"

	pubsubv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/operations"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/topic/resources"

	. "github.com/knative/pkg/reconciler/testing"

	. "github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/testing"
)

const (
	topicName = "hubbub"
	sinkName  = "sink"

	testNS = "testnamespace"

	testImage = "test_image"

	topicUID = topicName + "-abc-123"

	testProject        = "test-project-id"
	testTopicID        = "cloud-run-topic-" + testNS + "-" + topicName + "-" + topicUID
	testSubscriptionID = "cloud-run-topic-" + testNS + "-" + topicName + "-" + topicUID
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
		Name: "verify topic",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicPropagationPolicy("RestrictCreateRestrictDelete"),
			),
			newSink(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicPropagationPolicy("RestrictCreateRestrictDelete"),
				// Updates
				WithInitTopicConditions,
				WithTopicMarkTopicVerifying(testTopicID),
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), operations.ActionExists),
		},
	}, {
		Name: "create topic",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
			),
			newSink(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				// Updates
				WithInitTopicConditions,
				WithTopicMarkTopicCreating(testTopicID),
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), operations.ActionCreate),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, true),
		},
	}, {
		Name: "successful create",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithInitTopicConditions,
				WithTopicTopic(testTopicID),
			),
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), operations.ActionCreate),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithInitTopicConditions,
				WithTopicTopic(testTopicID),
				// Updates
				WithTopicDeployed,
			),
		}},
		WantCreates: []runtime.Object{
			newPubslicher(),
		},
	}, {
		Name: "successful create - reuse existing receive adapter",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithInitTopicConditions,
				WithTopicTopic(testTopicID),
			),
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), operations.ActionCreate),
			newPubslicher(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithInitTopicConditions,
				// Updates
				WithTopicReady(testTopicID),
			),
		}},
	}, {
		Name: "deleting - delete topic - policy CreateRestrictDelete",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, false),
		},
	}, {
		Name: "deleting - delete topic - policy CreateDelete",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
				// Updates
				WithTopicTopicDeleting(testTopicID),
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), operations.ActionDelete),
		},
	}, {
		Name: "deleting final stage - policy CreateDelete",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
				WithTopicTopicDeleting(testTopicID),
			),
			newTopicJobFinished(NewTopic(topicName, testNS, WithTopicUID(topicUID)), operations.ActionDelete, true),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project:            testProject,
					Topic:              testTopicID,
					ServiceAccountName: testServiceAccount,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
				// Updates
				WithTopicTopicDeleted(testTopicID),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, false),
		},
	},

	// TODO: subscriptions.
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		pubsubBase := &pubsub.PubSubBase{
			Base:          reconciler.NewBase(ctx, controllerAgentName, cmw),
			TopicOpsImage: testImage + "pub",
		}
		return &Reconciler{
			PubSubBase:       pubsubBase,
			deploymentLister: listers.GetDeploymentLister(),
			topicLister:      listers.GetTopicLister(),
			publisherImage:   testImage,
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

func newPubslicher() runtime.Object {
	topic := NewTopic(topicName, testNS,
		WithTopicUID(topicUID),
		WithTopicSpec(pubsubv1alpha1.TopicSpec{
			Project:            testProject,
			Topic:              testTopicID,
			ServiceAccountName: testServiceAccount,
		}))
	args := &resources.PublisherArgs{
		Image:  testImage,
		Topic:  topic,
		Labels: resources.GetLabels(controllerAgentName, topicName),
	}
	return resources.MakePublisher(args)
}

func newTopicJob(owner kmeta.OwnerRefable, action string) runtime.Object {
	return operations.NewTopicOps(operations.TopicArgs{
		Image:     testImage + "pub",
		Action:    action,
		ProjectID: testProject,
		TopicID:   testTopicID,
		Owner:     owner,
	})
}

func newTopicJobFinished(owner kmeta.OwnerRefable, action string, success bool) runtime.Object {
	job := operations.NewTopicOps(operations.TopicArgs{
		Image:     testImage + "pub",
		Action:    action,
		ProjectID: testProject,
		TopicID:   testTopicID,
		Owner:     owner,
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
