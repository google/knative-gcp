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

package intevents

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"

	logtesting "knative.dev/pkg/logging/testing"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/fake"
	testingmetadata "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	"github.com/google/knative-gcp/pkg/reconciler"
	rectesting "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS                                     = "test-namespace"
	name                                       = "obj-name"
	testTopicID                                = "topic"
	testProjectID                              = "project"
	receiveAdapterName                         = "test-receive-adapter"
	resourceGroup                              = "test-resource-group"
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
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
	pubsubable = rectesting.NewCloudStorageSource(name, testNS)

	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())
)

// Returns an ownerref for the test object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "CloudStorageSource",
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
		expectedTopic *inteventsv1alpha1.Topic
		expectedPS    *inteventsv1alpha1.PullSubscription
		expectedErr   string
		wantCreates   []runtime.Object
	}{{
		name: "topic does not exist, created, not yet been reconciled",
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicAnnotations(map[string]string{
				v1alpha1.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q has not yet been reconciled", name),
		wantCreates: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicAnnotations(map[string]string{
					v1alpha1.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		name: "topic exists but is not yet been reconciled",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q has not yet been reconciled", name),
	}, {
		name: "topic exists and is ready but no projectid",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q did not expose projectid", name),
	}, {
		name: "topic exists and the status of topic is false",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicFailed(),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicFailed(),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("the status of Topic %q is False", name),
	}, {
		name: "topic exists and the status of topic is unknown",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicUnknown(),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicUnknown(),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("the status of Topic %q is Unknown", name),
	}, {
		name: "topic exists and is ready but no topicid",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(""),
				rectesting.WithTopicAddress(testTopicURI),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicReady(""),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q did not expose topicid", name),
	}, {
		name: "topic exists and is ready, pullsubscription created, not yet been reconciled",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicAnnotations(map[string]string{
					v1alpha1.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicAnnotations(map[string]string{
				v1alpha1.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1alpha1.PubSubSpec{
					Secret: &secret,
				},
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group":       resourceGroup,
				v1alpha1.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedErr: fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToPropagatePullSubscriptionStatusMsg, name),
		wantCreates: []runtime.Object{
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1alpha1.PubSubSpec{
						Secret: &secret,
					},
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group":       resourceGroup,
					v1alpha1.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		name: "topic exists and is ready, pullsubscription exists, not yet been reconciled",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1alpha1.PubSubSpec{
						Secret: &secret,
					},
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1alpha1.PubSubSpec{
					Secret: &secret,
				},
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedErr: fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToPropagatePullSubscriptionStatusMsg, name),
	}, {
		name: "topic exists and is ready, pullsubscription exists and the status is false",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1alpha1.PubSubSpec{
						Secret: &secret,
					},
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithPullSubscriptionFailed(),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1alpha1.PubSubSpec{
					Secret: &secret,
				},
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithPullSubscriptionFailed(),
		),
		expectedErr: fmt.Sprintf("%s: the status of PullSubscription %q is False", failedToPropagatePullSubscriptionStatusMsg, name),
	}, {
		name: "topic exists and is ready, pullsubscription exists and the status is unknown",
		objects: []runtime.Object{
			rectesting.NewTopic(name, testNS,
				rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				rectesting.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithTopicProjectID(testProjectID),
				rectesting.WithTopicReady(testTopicID),
				rectesting.WithTopicAddress(testTopicURI),
			),
			rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
				rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1alpha1.PubSubSpec{
						Secret: &secret,
					},
				}),
				rectesting.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				rectesting.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				rectesting.WithPullSubscriptionUnknown(),
			),
		},
		expectedTopic: rectesting.NewTopic(name, testNS,
			rectesting.WithTopicSpec(inteventsv1alpha1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
			}),
			rectesting.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithTopicReady(testTopicID),
			rectesting.WithTopicProjectID(testProjectID),
			rectesting.WithTopicAddress(testTopicURI),
			rectesting.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedPS: rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
			rectesting.WithPullSubscriptionSpecWithNoDefaults(inteventsv1alpha1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1alpha1.PubSubSpec{
					Secret: &secret,
				},
			}),
			rectesting.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			rectesting.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			rectesting.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			rectesting.WithPullSubscriptionUnknown(),
		),
		expectedErr: fmt.Sprintf("%s: the status of PullSubscription %q is Unknown", failedToPropagatePullSubscriptionStatusMsg, name),
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := fakePubsubClient.NewSimpleClientset(tc.objects...)

			psBase := &PubSubBase{
				Base:               &reconciler.Base{},
				pubsubClient:       cs,
				receiveAdapterName: receiveAdapterName,
			}
			psBase.Logger = logtesting.TestLogger(t)

			arl := pkgtesting.ActionRecorderList{cs}
			topic, ps, err := psBase.ReconcilePubSub(context.Background(), pubsubable, testTopicID, resourceGroup)

			if (tc.expectedErr != "" && err == nil) ||
				(tc.expectedErr == "" && err != nil) ||
				(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
				t.Errorf("Test case %q, Error mismatch, want: %q got: %q", tc.name, tc.expectedErr, err)
			}
			if diff := cmp.Diff(tc.expectedTopic, topic, ignoreLastTransitionTime); diff != "" {
				t.Errorf("Test case %q, unexpected topic (-want, +got) = %v", tc.name, diff)
			}
			if diff := cmp.Diff(tc.expectedPS, ps, ignoreLastTransitionTime); diff != "" {
				t.Errorf("Test case %q, unexpected pullsubscription (-want, +got) = %v", tc.name, diff)
			}

			// Validate creates.
			actions, err := arl.ActionsByVerb()
			if err != nil {
				t.Errorf("Error capturing actions by verb: %q", err)
			}

			verifyCreateActions(t, actions.Creates, tc.wantCreates)
		})
	}
}

func TestDeletesOldPubSub(t *testing.T) {
	testCases := []struct {
		name        string
		objects     []runtime.Object
		expectedErr string
		wantUpdates []runtime.Object
		wantDeletes []clientgotesting.DeleteActionImpl
	}{
		{
			name: "old topic exists - CreateDelete",
			objects: []runtime.Object{
				rectesting.NewPubSubTopic(name, testNS,
					rectesting.WithPubSubTopicSpec(pubsubv1alpha1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
					}),
					rectesting.WithPubSubTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			wantUpdates: []runtime.Object{
				rectesting.NewPubSubTopic(name, testNS,
					rectesting.WithPubSubTopicSpec(pubsubv1alpha1.TopicSpec{
						Topic: testTopicID,
						// Policy is changed to NoDelete version.
						PropagationPolicy: "CreateNoDelete",
					}),
					rectesting.WithPubSubTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource: schema.GroupVersionResource{
							Group:    "pubsub.cloud.google.com",
							Version:  "v1alpha1",
							Resource: "topics",
						},
					},
					Name: name,
				},
			},
		},
		{
			name: "old topic exists - CreateNoDelete",
			objects: []runtime.Object{
				rectesting.NewPubSubTopic(name, testNS,
					rectesting.WithPubSubTopicSpec(pubsubv1alpha1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateNoDelete",
					}),
					rectesting.WithPubSubTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource: schema.GroupVersionResource{
							Group:    "pubsub.cloud.google.com",
							Version:  "v1alpha1",
							Resource: "topics",
						},
					},
					Name: name,
				},
			},
		},
		{
			name: "old topic exists - NoCreateNoDelete",
			objects: []runtime.Object{
				rectesting.NewPubSubTopic(name, testNS,
					rectesting.WithPubSubTopicSpec(pubsubv1alpha1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "NoCreateNoDelete",
					}),
					rectesting.WithPubSubTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource: schema.GroupVersionResource{
							Group:    "pubsub.cloud.google.com",
							Version:  "v1alpha1",
							Resource: "topics",
						},
					},
					Name: name,
				},
			},
		},
		{
			name: "old topic exists - Bad Propagation Policy",
			objects: []runtime.Object{
				rectesting.NewPubSubTopic(name, testNS,
					rectesting.WithPubSubTopicSpec(pubsubv1alpha1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "bad-policy",
					}),
					rectesting.WithPubSubTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			expectedErr: "unknown propagation policy on old Topic: bad-policy",
		},
		{
			name: "old topic exists - Not owned by the Pubsubable",
			objects: []runtime.Object{
				rectesting.NewPubSubTopic(name, testNS,
					rectesting.WithPubSubTopicSpec(pubsubv1alpha1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "bad-policy",
					}),
				),
			},
		},
		{
			name: "old PullSubscription exists - Not Owned by Pubsubable",
			objects: []runtime.Object{
				rectesting.NewPubSubPullSubscription(name, testNS),
			},
		},
		{
			name: "old PullSubscription exists - Deleted",
			objects: []runtime.Object{
				rectesting.NewPubSubPullSubscription(name, testNS,
					rectesting.WithPubSubPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			wantDeletes: []clientgotesting.DeleteActionImpl{
				{
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Resource: schema.GroupVersionResource{
							Group:    "pubsub.cloud.google.com",
							Version:  "v1alpha1",
							Resource: "pullsubscriptions",
						},
					},
					Name: name,
				},
			},
		},
	}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects := append(tc.objects, []runtime.Object{
				// Ready Topic.
				rectesting.NewTopic(name, testNS,
					rectesting.WithTopicProjectID(testProjectID),
					rectesting.WithTopicReady(testTopicID),
					rectesting.WithTopicAddress(testTopicURI),
				),
				// Ready PullSubscription
				rectesting.NewPullSubscriptionWithNoDefaults(name, testNS,
					rectesting.WithPullSubscriptionReady(apis.HTTP("foo")),
				),
			}...)
			cs := fakePubsubClient.NewSimpleClientset(objects...)

			psBase := &PubSubBase{
				Base: &reconciler.Base{
					Recorder: &record.FakeRecorder{},
				},
				pubsubClient:       cs,
				receiveAdapterName: receiveAdapterName,
			}
			psBase.Logger = logtesting.TestLogger(t)

			arl := pkgtesting.ActionRecorderList{cs}
			_, _, err := psBase.ReconcilePubSub(context.Background(), pubsubable, testTopicID, resourceGroup)

			if (tc.expectedErr != "" && err == nil) ||
				(tc.expectedErr == "" && err != nil) ||
				(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
				t.Errorf("Test case %q, Error mismatch, want: %q got: %q", tc.name, tc.expectedErr, err)
			}

			actions, err := arl.ActionsByVerb()
			if err != nil {
				t.Errorf("Error capturing actions by verb: %q", err)
			}

			verifyUpdateActions(t, actions.Updates, tc.wantUpdates)
			verifyDeleteActions(t, actions.Deletes, tc.wantDeletes)
		})
	}
}

func verifyCreateActions(t *testing.T, actual []clientgotesting.CreateAction, expected []runtime.Object) {
	for i, want := range expected {
		if i >= len(actual) {
			t.Errorf("Missing create: %#v", want)
			continue
		}
		got := actual[i]
		obj := got.GetObject()
		if diff := cmp.Diff(want, obj); diff != "" {
			t.Errorf("Unexpected create (-want, +got): %s", diff)
		}
	}
	if got, want := len(actual), len(expected); got > want {
		for _, extra := range actual[want:] {
			t.Errorf("Extra create: %#v", extra.GetObject())
		}
	}
}

func verifyUpdateActions(t *testing.T, actual []clientgotesting.UpdateAction, expected []runtime.Object) {
	for i, want := range expected {
		if i >= len(actual) {
			t.Errorf("Missing update: %#v", want)
			continue
		}
		got := actual[i]
		obj := got.GetObject()
		if diff := cmp.Diff(want, obj); diff != "" {
			t.Errorf("Unexpected update (-want, +got): %s", diff)
		}
	}
	if got, want := len(actual), len(expected); got > want {
		for _, extra := range actual[want:] {
			t.Errorf("Extra update: %#v", extra.GetObject())
		}
	}
}

func verifyDeleteActions(t *testing.T, actual []clientgotesting.DeleteAction, expected []clientgotesting.DeleteActionImpl) {
	for i, want := range expected {
		if i >= len(actual) {
			t.Errorf("Missing delete: %#v", want)
			continue
		}
		got := actual[i]

		wantGVR := want.GetResource()
		gotGVR := got.GetResource()
		if diff := cmp.Diff(wantGVR, gotGVR); diff != "" {
			t.Errorf("Unexpected delete GVR (-want +got): %s", diff)
		}
		if w, g := want.Namespace, got.GetNamespace(); w != g {
			t.Errorf("Unexpected delete namespace. Expected %q, actually %q", w, g)
		}
		if w, g := want.Name, got.GetName(); w != g {
			t.Errorf("Unexpected delete name. Expected %q, actually %q", w, g)
		}
	}
	if got, want := len(actual), len(expected); got > want {
		for _, extra := range actual[want:] {
			t.Errorf("Extra delete: %#v", extra)
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
					Resource:  schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1alpha1", Resource: "topics"},
				},
				Name: name,
			}, {
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Verb:      "delete",
					Resource:  schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"},
				},
				Name: name,
			},
		},
	}}

	defer logtesting.ClearAll()

	for _, tc := range testCases {
		cs := fakePubsubClient.NewSimpleClientset()

		psBase := &PubSubBase{
			Base:               &reconciler.Base{},
			pubsubClient:       cs,
			receiveAdapterName: receiveAdapterName,
		}
		psBase.Logger = logtesting.TestLogger(t)

		arl := pkgtesting.ActionRecorderList{cs}
		err := psBase.DeletePubSub(context.Background(), pubsubable)

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
