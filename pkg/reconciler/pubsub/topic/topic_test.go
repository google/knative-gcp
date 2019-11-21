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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

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

	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	ops "github.com/google/knative-gcp/pkg/operations"
	operations "github.com/google/knative-gcp/pkg/operations/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/events/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/topic/resources"

	. "knative.dev/pkg/reconciler/testing"

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

	secretName            = "testing-secret"
	testJobFailureMessage = "job failed"
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
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				WithInitTopicConditions,
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionExists),
		},
	}, {
		Name: "create topic",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
			),
			newSink(),
			newSecret(),
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				// Updates
				WithInitTopicConditions,
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionCreate),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, finalizerName),
		},
	}, {
		Name: "failed to create topic",
		Objects: append([]runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitTopicConditions,
			)},
			newTopicJobFinished(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionCreate, false)...,
		),
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", testJobFailureMessage),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitTopicConditions,
				// Updates
				WithTopicJobFailure(testTopicID, "CreateFailed", fmt.Sprintf("Failed to create Topic: %q", testJobFailureMessage)),
			),
		}},
		WantErr: true,
	}, {
		Name: "successful create",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitTopicConditions,
				WithTopicTopicID(testTopicID),
			),
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionCreate),
			newSecret(),
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitTopicConditions,
				WithTopicTopicID(testTopicID),
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithInitTopicConditions,
				WithTopicTopicID(testTopicID),
			),
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionCreate),
			newSecret(),
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, ""),
			patchFinalizers(testNS, secretName, "", "noisy-finalizer"),
		},
	}, {
		Name: "deleting - delete topic - policy CreateDelete",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
		}},
		WantCreates: []runtime.Object{
			newTopicJob(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionDelete),
		},
	}, {
		Name: "skip deleting if topic not exists - policy CreateDelete",
		Objects: []runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicJobFailure(testTopicID, "CreateFailed", "topic creation failed"),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
			newSecret(),
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
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithInitTopicConditions,
				WithTopicJobFailure(testTopicID, "CreateFailed", "topic creation failed"),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, ""),
			patchFinalizers(testNS, secretName, "", "noisy-finalizer"),
		},
	}, {
		Name: "deleting final stage - policy CreateDelete",
		Objects: append([]runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
			newSecret()},
			newTopicJobFinished(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionDelete, true)...,
		),
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
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
			patchFinalizers(testNS, topicName, ""),
			patchFinalizers(testNS, secretName, "", "noisy-finalizer"),
		},
	}, {
		Name: "deleting final stage - policy CreateDelete - not the only Topic",
		Objects: append([]runtime.Object{
			NewTopic("not-relevant", testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
			),
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			),
			newSecret()},
			newTopicJobFinished(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionDelete, true)...,
		),
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Topic %q", topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
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
			patchFinalizers(testNS, topicName, ""),
		},
	}, {
		Name: "fail to delete topic - policy CreateDelete",
		Objects: append([]runtime.Object{
			NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
			)},
			newTopicJobFinished(NewTopic(topicName, testNS, WithTopicUID(topicUID)), ops.ActionDelete, false)...,
		),
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", testJobFailureMessage),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewTopic(topicName, testNS,
				WithTopicUID(topicUID),
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				WithTopicPropagationPolicy("CreateDelete"),
				WithTopicReady(testTopicID),
				WithTopicFinalizers(finalizerName),
				WithTopicDeleted,
				// Updates
				WithTopicJobFailure(testTopicID, "DeleteFailed", fmt.Sprintf("Failed to delete topic: %q.", testJobFailureMessage)),
			),
		}},
		WantErr: true,
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
			serviceLister:  listers.GetV1ServiceLister(),
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

func newPublisher(get, done bool) runtime.Object {
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
	pub := resources.MakePublisher(args)
	if get {
		if done {
			pub.Status.Conditions = []apis.Condition{{
				Type:   apis.ConditionReady,
				Status: "True",
			}}
			uri, _ := apis.ParseURL(testTopicURI)
			pub.Status.Address = &duckv1.Addressable{
				URL: uri,
			}
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
		Secret:    secret,
		Owner:     owner,
	})
}

func newTopicJobFinished(owner kmeta.OwnerRefable, action string, success bool) []runtime.Object {
	job := operations.NewTopicOps(operations.TopicArgs{
		Image:     testImage + "pub",
		Action:    action,
		ProjectID: testProject,
		TopicID:   testTopicID,
		Secret:    secret,
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

	podTerminationMessage := fmt.Sprintf(`{"projectId":"%s"}`, testProject)
	if !success {
		podTerminationMessage = fmt.Sprintf(`{"projectId":"%s","reason":"%s"}`, testProject, testJobFailureMessage)
	}

	jobPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pubsub-s-source-topic-create-pod",
			Namespace: testNS,
			Labels:    map[string]string{"job-name": job.Name},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "job",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  podTerminationMessage,
						},
					},
				},
			},
		},
	}

	return []runtime.Object{job, jobPod}
}
