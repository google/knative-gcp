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

package reconciler

import (
	"context"
	//	"encoding/json"
	//	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	//	"knative.dev/pkg/kmeta"

	"github.com/google/go-cmp/cmp"
	//	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	//	clientgotesting "k8s.io/client-go/testing"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	//	"knative.dev/pkg/configmap"
	//	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	pubsubsourcev1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	//	schedulerv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	//	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	//fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/pubsub/v1alpha1/fake"
	fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/fake"
	//	ops "github.com/google/knative-gcp/pkg/operations"
	//	operations "github.com/google/knative-gcp/pkg/operations/scheduler"
	rectesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

const (
	testNS             = "test-namespace"
	name               = "obj-name"
	testGroup          = "testgroup"
	testVersion        = "v1alpha17"
	testKind           = "Testkind"
	testTopicID        = "topic"
	receiveAdapterName = "test-receive-adapter"
)

var (
	trueVal = true

	testTopicURI = "http://" + name + "-topic." + testNS + ".svc.cluster.local"

	secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "google-cloud-key",
		},
		Key: "key.json",
	}
	spec = &duckv1alpha1.PubSubSpec{
		Secret: &secret,
	}
	status = &duckv1alpha1.PubSubStatus{}
)

type testOwnerRefable struct {
	metav1.ObjectMeta
}

func (tor *testOwnerRefable) GetGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: testGroup, Version: testVersion, Kind: testKind}
}

// Returns an ownerref for the test object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "testgroup/v1alpha17",
		Kind:       testKind,
		Name:       name,
		//		UID:                schedulerUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}

func TestAllCases(t *testing.T) {

	testCases := []struct {
		name          string
		objects       []runtime.Object
		expectedTopic *pubsubsourcev1alpha1.Topic
		expectedPS    *pubsubsourcev1alpha1.PullSubscription
		expectedErr   string
		wantCreates   []runtime.Object
	}{{
		name: "topic does not exist, created, not ready",
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(pubsubsourcev1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter": receiveAdapterName,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: "topic not ready",
		wantCreates: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(pubsubsourcev1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		name: "topic exists and is ready, pull subscription created",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(pubsubsourcev1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(pubsubsourcev1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter": receiveAdapterName,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: "topic not ready",
		wantCreates: []runtime.Object{
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS),
		},
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		cs := fakePubsubClient.NewSimpleClientset(tc.objects...)
		//		_ = rectesting.NewListers(tc.objects)

		psBase := &PubSubBase{
			Base:               &Base{},
			pubsubClient:       cs,
			receiveAdapterName: receiveAdapterName,
		}
		psBase.Logger = logtesting.TestLogger(t)

		arl := pkgtesting.ActionRecorderList{cs}
		condSet := &apis.ConditionSet{}
		topic, ps, err := psBase.ReconcilePubSub(context.Background(), testNS, name, spec, status, condSet, &testOwnerRefable{metav1.ObjectMeta{Namespace: testNS, Name: name}}, testTopicID)

		if (tc.expectedErr != "" && err == nil) ||
			(tc.expectedErr == "" && err != nil) ||
			(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
			t.Errorf("Error mismatch, want: %q got: %q", tc.expectedErr, err)
		}
		if diff := cmp.Diff(tc.expectedTopic, topic); diff != "" {
			t.Errorf("unexpected topic (-want, +got) = %v", diff)
		}
		if diff := cmp.Diff(tc.expectedPS, ps); diff != "" {
			t.Errorf("unexpected pullsubscription (-want, +got) = %v", diff)
		}

		// Validate creates.
		actions, err := arl.ActionsByVerb()
		if err != nil {
			t.Errorf("Error capturing actions by verb: %q", err)
		}
		for i, want := range tc.wantCreates {
			if i >= len(actions.Creates) {
				t.Errorf("Missing create: %#v", want)
				continue
			}
			got := actions.Creates[i]
			obj := got.GetObject()
			if diff := cmp.Diff(want, obj); diff != "" {
				t.Errorf("Unexpected create (-want, +got): %s", diff)
			}
		}
		if got, want := len(actions.Creates), len(tc.wantCreates); got > want {
			for _, extra := range actions.Creates[want:] {
				t.Errorf("Extra create: %#v", extra.GetObject())
			}
		}
	}

}
