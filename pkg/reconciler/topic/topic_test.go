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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/kmeta"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	pubsubv1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/operations"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsub"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/topic/resources"

	. "knative.dev/pkg/reconciler/testing"

	. "github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/testing"
)

const (
	topicName = "hubbub"
	sinkName  = "sink"

	testNS             = "testnamespace"
	testImage          = "test_image"
	topicUID           = topicName + "-abc-123"
	testProject        = "test-project-id"
	testTopicID        = "cloud-run-topic-" + testNS + "-" + topicName + "-" + topicUID
	testServiceAccount = "test-project-id"
	testTopicURI       = "http://" + topicName + "-topic." + testNS + ".svc.cluster.local"
)

var (
	trueVal = true

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
				WithTopicPropagationPolicy("NoCreateNoDelete"),
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
				WithTopicPropagationPolicy("NoCreateNoDelete"),
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
		WithReactors: []clientgotesting.ReactionFunc{
			ProvideResource("create", "services", newPublisher(true, true)),
		},
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
				WithTopicAddress(testTopicURI),
			),
		}},
		WantCreates: []runtime.Object{
			newPublisher(false, false),
		},
	}, {
		Name: "successful create - reuse existing publisher",
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
			newPublisher(true, true),
			NewService(topicName+"-topic", testNS,
				WithServiceOwnerReferences(ownerReferences()),
				WithServiceLabels(resources.GetLabels(controllerAgentName, topicName)),
				WithServicePorts(servicePorts())),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WithReactors: []clientgotesting.ReactionFunc{},
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
				WithTopicAddress(testTopicURI),
			),
		}},
	}, {
		Name: "deleting - delete topic - policy CreateNoDelete",
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
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		pubsubBase := &pubsub.PubSubBase{
			Base:          reconciler.NewBase(ctx, controllerAgentName, cmw),
			TopicOpsImage: testImage + "pub",
		}
		return &Reconciler{
			PubSubBase:     pubsubBase,
			topicLister:    listers.GetTopicLister(),
			serviceLister:  listers.GetServiceLister(),
			publisherImage: testImage,
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

func ownerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         "pubsub.cloud.run/v1alpha1",
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

func newPublisher(get, done bool) runtime.Object {
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
	pub := resources.MakePublisher(args)
	if get {
		if done {
			pub.Status.Conditions = []apis.Condition{{
				Type:   apis.ConditionReady,
				Status: "True",
			}}
			uri, _ := apis.ParseURL(testTopicURI)
			pub.Status.Address = &v1beta1.Addressable{URL: uri}
		} else {
			pub.Status.Conditions = []apis.Condition{{
				Type:   apis.ConditionReady,
				Status: "Unknown",
			}}
		}
	}
	return pub
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
