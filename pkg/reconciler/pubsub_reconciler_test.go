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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	logtesting "knative.dev/pkg/logging/testing"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	pubsubsourcev1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/fake"
	rectesting "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS             = "test-namespace"
	name               = "obj-name"
	testTopicID        = "topic"
	testProjectID      = "project"
	receiveAdapterName = "test-receive-adapter"
	resourceGroup      = "test-resource-group"
	sinkName           = "sink"
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
	pubsubable = rectesting.NewStorage(name, testNS)

	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())
)

// Returns an ownerref for the test object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "Storage",
		Name:               name,
		UID:                "test-storage-uid",
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}

func TestCreates(t *testing.T) {
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
		name: "topic exists but is not ready",
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
	}, {
		name: "topic exists and is ready but no projectid",
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
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: "topic did not expose projectid",
	}, {
		name: "topic exists and is ready but no topicid",
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
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(""),
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
			rectesting.WithTopicReady(""),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: "topic did not expose topicid",
	}, {
		name: "topic exists and is ready, pullsubscription created",
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
				rectesting.WithTopicProjectID(testProjectID),
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
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(pubsubsourcev1alpha1.PullSubscriptionSpec{
				Topic:  testTopicID,
				Secret: &secret,
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter": receiveAdapterName,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedErr: "pullsubscription not ready",
		wantCreates: []runtime.Object{
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(pubsubsourcev1alpha1.PullSubscriptionSpec{
					Topic:  testTopicID,
					Secret: &secret,
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		name: "topic exists and is ready, pullsubscription exists, not ready",
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
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(pubsubsourcev1alpha1.PullSubscriptionSpec{
					Topic:  testTopicID,
					Secret: &secret,
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
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
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(pubsubsourcev1alpha1.PullSubscriptionSpec{
				Topic:  testTopicID,
				Secret: &secret,
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter": receiveAdapterName,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),

		expectedErr: "pullsubscription not ready",
	}, {
		name: "topic exists and is ready, pullsubscription exists and is ready",
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
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(pubsubsourcev1alpha1.PullSubscriptionSpec{
					Topic:  testTopicID,
					Secret: &secret,
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithPullSubscriptionReady("http://example.com"),
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
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(pubsubsourcev1alpha1.PullSubscriptionSpec{
				Topic:  testTopicID,
				Secret: &secret,
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter": receiveAdapterName,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithPullSubscriptionReady("http://example.com"),
		),

		expectedErr: "",
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		cs := fakePubsubClient.NewSimpleClientset(tc.objects...)

		psBase := &PubSubBase{
			Base:               &Base{},
			pubsubClient:       cs,
			receiveAdapterName: receiveAdapterName,
		}
		psBase.Logger = logtesting.TestLogger(t)

		arl := pkgtesting.ActionRecorderList{cs}
		topic, ps, err := psBase.ReconcilePubSub(context.Background(), pubsubable, testTopicID, resourceGroup)

		if (tc.expectedErr != "" && err == nil) ||
			(tc.expectedErr == "" && err != nil) ||
			(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
			t.Errorf("Error mismatch, want: %q got: %q", tc.expectedErr, err)
		}
		if diff := cmp.Diff(tc.expectedTopic, topic, ignoreLastTransitionTime); diff != "" {
			t.Errorf("unexpected topic (-want, +got) = %v", diff)
		}
		if diff := cmp.Diff(tc.expectedPS, ps, ignoreLastTransitionTime); diff != "" {
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

func TestDeletes(t *testing.T) {
	testCases := []struct {
		name        string
		wantDeletes []clientgotesting.DeleteActionImpl
		expectedErr string
	}{{
		name:        "topic and pullsubscription deleeted",
		expectedErr: "",
		wantDeletes: []clientgotesting.DeleteActionImpl{
			{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Verb:      "delete",
					Resource:  schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"},
				},
				Name: name,
			}, {
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Verb:      "delete",
					Resource:  schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"},
				},
				Name: name,
			},
		},
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		cs := fakePubsubClient.NewSimpleClientset()

		psBase := &PubSubBase{
			Base:               &Base{},
			pubsubClient:       cs,
			receiveAdapterName: receiveAdapterName,
		}
		psBase.Logger = logtesting.TestLogger(t)

		arl := pkgtesting.ActionRecorderList{cs}
		err := psBase.DeletePubSub(context.Background(), testNS, name)

		if (tc.expectedErr != "" && err == nil) ||
			(tc.expectedErr == "" && err != nil) ||
			(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
			t.Errorf("Error mismatch, want: %q got: %q", tc.expectedErr, err)
		}

		// validate deletes
		actions, err := arl.ActionsByVerb()
		if err != nil {
			t.Errorf("Error capturing actions by verb: %q", err)
		}
		for i, want := range tc.wantDeletes {
			if i >= len(actions.Deletes) {
				t.Errorf("Missing delete: %#v", want)
				continue
			}
			got := actions.Deletes[i]
			if got.GetName() != want.GetName() {
				t.Errorf("Unexpected delete[%d]: %#v", i, got)
			}
			if got.GetResource() != want.GetResource() {
				t.Errorf("Unexpected delete[%d]: %#v wanted: %#v", i, got, want)
			}
		}
		if got, want := len(actions.Deletes), len(tc.wantDeletes); got > want {
			for _, extra := range actions.Deletes[want:] {
				t.Errorf("Extra delete: %s/%s", extra.GetNamespace(), extra.GetName())
			}
		}
	}
}
