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
	"path"
	"reflect"
	"strings"
	"testing"

	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/duck"
	v1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	intereventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/fake"
	testingmetadata "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	"github.com/google/knative-gcp/pkg/reconciler"
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

	oldSecret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "old-secret",
		},
		Key: "key.json",
	}

	sink = duckv1.Destination{
		URI: apis.HTTP("sink"),
	}

	oldSink = duckv1.Destination{
		URI: apis.HTTP("oldSink"),
	}

	pubsubable = reconcilertestingv1.NewCloudStorageSource(name, testNS,
		reconcilertestingv1.WithCloudStorageSourceSinkDestination(sink),
		reconcilertestingv1.WithCloudStorageSourceSetDefaults)

	ignoreLastTransitionTime = cmp.FilterPath(func(p cmp.Path) bool {
		return strings.HasSuffix(p.String(), "LastTransitionTime.Inner.Time")
	}, cmp.Ignore())

	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)

// Returns an ownerref for the test object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1",
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
		expectedTopic *intereventsv1.Topic
		expectedPS    *intereventsv1.PullSubscription
		expectedErr   string
		wantCreates   []runtime.Object
		wantUpdates   []clientgotesting.UpdateActionImpl
	}{{
		name: "topic does not exist, created, not yet been reconciled",
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicAnnotations(map[string]string{
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q has not yet been reconciled", name),
		wantCreates: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Secret:            &secret,
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
	}, {
		name: "topic exists but is not yet been reconciled",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q has not yet been reconciled", name),
	}, {
		name: "topic exists and is ready but no projectid",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q did not expose projectid", name),
	}, {
		name: "topic exists and the status of topic is false",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicFailed,
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicFailed,
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("the status of Topic %q is False", name),
	}, {
		name: "topic exists and the status of topic is unknown",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicUnknown,
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicUnknown,
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("the status of Topic %q is Unknown", name),
	}, {
		name: "topic exists and is ready but no topicid",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(""),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(""),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS:  nil,
		expectedErr: fmt.Sprintf("Topic %q did not expose topicid", name),
	}, {
		name: "topic updated due to different secret, pullsubscription created, not yet been reconciled",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Secret:            &oldSecret,
					Topic:             testTopicID,
					PropagationPolicy: "CreatDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicAnnotations(map[string]string{
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS: reconcilertestingv1.NewPullSubscription(name, testNS,
			reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1.PubSubSpec{
					Secret: &secret,
					SourceSpec: duckv1.SourceSpec{
						Sink: sink,
					},
				},
			}),
			reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group":   resourceGroup,
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedErr: fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToPropagatePullSubscriptionStatusMsg, name),
		wantCreates: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: sink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group":   resourceGroup,
					duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		wantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Secret:            &secret,
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
	}, {
		name: "topic exists and is ready, pullsubscription created, not yet been reconciled",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicAnnotations(map[string]string{
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS: reconcilertestingv1.NewPullSubscription(name, testNS,
			reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1.PubSubSpec{
					Secret: &secret,
					SourceSpec: duckv1.SourceSpec{
						Sink: sink,
					},
				},
			}),
			reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group":   resourceGroup,
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			}),
			reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedErr: fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToPropagatePullSubscriptionStatusMsg, name),
		wantCreates: []runtime.Object{
			reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: sink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group":   resourceGroup,
					duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		name: "topic exists and is ready, pullsubscription exists, not yet been reconciled",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: sink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS: reconcilertestingv1.NewPullSubscription(name, testNS,
			reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1.PubSubSpec{
					Secret: &secret,
					SourceSpec: duckv1.SourceSpec{
						Sink: sink,
					},
				},
			}),
			reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
		),
		expectedErr: fmt.Sprintf("%s: PullSubscription %q has not yet been reconciled", failedToPropagatePullSubscriptionStatusMsg, name),
	}, {
		name: "topic exists and is ready, pullsubscription exists and the status is false",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: sink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithPullSubscriptionFailed(),
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS: reconcilertestingv1.NewPullSubscription(name, testNS,
			reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1.PubSubSpec{
					Secret: &secret,
					SourceSpec: duckv1.SourceSpec{
						Sink: sink,
					},
				},
			}),
			reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithPullSubscriptionFailed(),
		),
		expectedErr: fmt.Sprintf("%s: the status of PullSubscription %q is False", failedToPropagatePullSubscriptionStatusMsg, name),
	}, {
		name: "topic exists and is ready, pullsubscription exists and the status is unknown",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: sink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithPullSubscriptionUnknown(),
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS: reconcilertestingv1.NewPullSubscription(name, testNS,
			reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1.PubSubSpec{
					Secret: &secret,
					SourceSpec: duckv1.SourceSpec{
						Sink: sink,
					},
				},
			}),
			reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithPullSubscriptionUnknown(),
		),
		expectedErr: fmt.Sprintf("%s: the status of PullSubscription %q is Unknown", failedToPropagatePullSubscriptionStatusMsg, name),
	}, {
		name: "topic exists and is ready, pullsubscription is updated due to different sink",
		objects: []runtime.Object{
			reconcilertestingv1.NewTopic(name, testNS,
				reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				reconcilertestingv1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithTopicProjectID(testProjectID),
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: oldSink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithPullSubscriptionReady(oldSink.URI),
			),
		},
		expectedTopic: reconcilertestingv1.NewTopic(name, testNS,
			reconcilertestingv1.WithTopicSpec(intereventsv1.TopicSpec{
				Secret:            &secret,
				Topic:             testTopicID,
				PropagationPolicy: "CreateDelete",
				EnablePublisher:   &falseVal,
			}),
			reconcilertestingv1.WithTopicLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
			reconcilertestingv1.WithTopicProjectID(testProjectID),
			reconcilertestingv1.WithTopicAddress(testTopicURI),
			reconcilertestingv1.WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			reconcilertestingv1.WithTopicSetDefaults,
		),
		expectedPS: reconcilertestingv1.NewPullSubscription(name, testNS,
			reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
				Topic: testTopicID,
				PubSubSpec: v1.PubSubSpec{
					Secret: &secret,
					SourceSpec: duckv1.SourceSpec{
						Sink: sink,
					},
				},
			}),
			reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
				"receive-adapter":                     receiveAdapterName,
				"events.cloud.google.com/source-name": name,
			}),
			reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
				"metrics-resource-group": resourceGroup,
			}),
			reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			// The SinkURI is the old one because we are not calling the PS reconciler in the UT.
			reconcilertestingv1.WithPullSubscriptionReady(oldSink.URI),
		),
		wantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewPullSubscription(name, testNS,
				reconcilertestingv1.WithPullSubscriptionSpec(intereventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: v1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: sink,
						},
					},
				}),
				reconcilertestingv1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": name,
				}),
				reconcilertestingv1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				reconcilertestingv1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				reconcilertestingv1.WithPullSubscriptionReady(oldSink.URI),
			),
		}},
	}}

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

func verifyUpdateActions(t *testing.T, actual []clientgotesting.UpdateAction, expected []runtime.Object, originalObjects []runtime.Object) {
	// Previous state is used to diff resource expected state for update requests that were missed.
	objPrevState := make(map[string]runtime.Object, len(originalObjects))
	for _, o := range originalObjects {
		objPrevState[objKey(o)] = o
	}

	updates := filterUpdatesWithSubresource("", actual)
	for i, want := range actual {
		if i >= len(updates) {
			wo := want.GetObject()
			key := objKey(wo)
			oldObj, ok := objPrevState[key]
			if !ok {
				t.Errorf("Object %s was never created: want: %#v", key, wo)
				continue
			}
			t.Errorf("Missing update for %s (-want, +prevState): %s", key,
				cmp.Diff(wo, oldObj, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()))
			continue
		}

		if want.GetSubresource() != "" {
			t.Errorf("Expectation was invalid - it should not include a subresource: %#v", want)
		}

		got := updates[i].GetObject()

		// Update the object state.
		objPrevState[objKey(got)] = got

		if diff := cmp.Diff(want.GetObject(), got, ignoreLastTransitionTime, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Unexpected update (-want, +got): %s", diff)
		}
	}
	if got, want := len(updates), len(actual); got > want {
		for _, extra := range updates[want:] {
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

// Code extracted from the the Table framework in knative.dev/pkg

func filterUpdatesWithSubresource(
	subresource string,
	actions []clientgotesting.UpdateAction) (result []clientgotesting.UpdateAction) {
	for _, action := range actions {
		if action.GetSubresource() == subresource {
			result = append(result, action)
		}
	}
	return
}

func objKey(o runtime.Object) string {
	on := o.(kmeta.Accessor)

	var typeOf string
	if gvk := on.GroupVersionKind(); gvk.Group != "" {
		// This must be populated if we're dealing with unstructured.Unstructured.
		typeOf = gvk.String()
	} else if or, ok := on.(kmeta.OwnerRefable); ok {
		// This is typically implemented by Knative resources.
		typeOf = or.GetGroupVersionKind().String()
	} else {
		// Worst case, fallback on a non-GVK string.
		typeOf = reflect.TypeOf(o).String()
	}

	// namespace + name is not unique, and the tests don't populate k8s kind
	// information, so use GoLang's type name as part of the key.
	return path.Join(typeOf, on.GetNamespace(), on.GetName())
}

func TestDeletes(t *testing.T) {
	testCases := []struct {
		name        string
		wantDeletes []clientgotesting.DeleteActionImpl
		expectedErr string
	}{{
		name:        "topic and pullsubscription deleted",
		expectedErr: "",
		wantDeletes: []clientgotesting.DeleteActionImpl{
			{
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Verb:      "delete",
					Resource:  schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1", Resource: "topics"},
				},
				Name: name,
			}, {
				ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS,
					Verb:      "delete",
					Resource:  schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1", Resource: "pullsubscriptions"},
				},
				Name: name,
			},
		},
	}}

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
