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

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	. "knative.dev/pkg/reconciler/testing"

	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	schedulerv1beta1 "github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	. "github.com/google/knative-gcp/pkg/apis/intevents"
	inteventsv1beta1 "github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1beta1/cloudschedulersource"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	gscheduler "github.com/google/knative-gcp/pkg/gclient/scheduler/testing"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	schedulerName = "my-test-scheduler"
	schedulerUID  = "test-scheduler-uid"
	sinkName      = "sink"

	testNS              = "testnamespace"
	testImage           = "scheduler-ops-image"
	testProject         = "test-project-id"
	testTopicURI        = "http://" + schedulerName + "-topic." + testNS + ".svc.cluster.local"
	location            = "us-central1"
	parentName          = "projects/" + testProject + "/locations/" + location
	jobName             = parentName + "/jobs/cre-scheduler-" + schedulerUID
	testData            = "mytestdata"
	onceAMinuteSchedule = "* * * * *"

	// Message for when the topic and pullsubscription with the above variables are not ready.
	failedToReconcileTopicMsg                  = `Topic has not yet been reconciled`
	failedToReconcilePullSubscriptionMsg       = `PullSubscription has not yet been reconciled`
	failedToReconcileJobMsg                    = `Failed to reconcile CloudSchedulerSource job`
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
	failedToDeleteJobMsg                       = `Failed to delete CloudSchedulerSource job`
)

var (
	trueVal  = true
	falseVal = false

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = apis.HTTP(sinkDNS)

	testTopicID = fmt.Sprintf("cre-src_%s_%s_%s", testNS, schedulerName, schedulerUID)

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1beta1",
		Kind:    "Sink",
	}

	secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "google-cloud-key",
		},
		Key: "key.json",
	}

	gServiceAccount = "test123@test123.iam.gserviceaccount.com"
)

func init() {
	// Add types to scheme
	_ = schedulerv1beta1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test CloudSchedulerSource object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1beta1",
		Kind:               "CloudSchedulerSource",
		Name:               "my-test-scheduler",
		UID:                schedulerUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}

func patchFinalizers(namespace, name string, add bool) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	var fname string
	if add {
		fname = fmt.Sprintf("%q", resourceGroup)
	}
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func newSink() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testing.cloud.google.com/v1beta1",
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

func newSinkDestination() duckv1.Destination {
	return duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "testing.cloud.google.com/v1beta1",
			Kind:       "Sink",
			Name:       sinkName,
		},
	}
}

// TODO add a unit test for successfully creating a k8s service account, after issue https://github.com/google/knative-gcp/issues/657 gets solved.
func TestAllCases(t *testing.T) {
	schedulerSinkURL := sinkURI

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "topic created, not ready",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceAnnotations(map[string]string{
					duckv1beta1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				WithCloudSchedulerSourceSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceAnnotations(map[string]string{
					duckv1beta1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				WithCloudSchedulerSourceTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
					SourceLabelKey:    schedulerName,
				}),
				WithTopicAnnotations(map[string]string{
					duckv1beta1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				WithTopicSetDefaults,
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: Topic %q has not yet been reconciled", schedulerName),
		},
	}, {
		Name: "topic exists, topic has not yet been reconciled",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceSetDefaults,
			),
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicUnknown,
				WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: the status of Topic %q is Unknown", schedulerName),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceSetDefaults,
			),
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceTopicFailed("TopicNotReady", `Topic "my-test-scheduler" did not expose projectid`),
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: Topic %q did not expose projectid", schedulerName),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceSetDefaults,
			),
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
				WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceTopicFailed("TopicNotReady", `Topic "my-test-scheduler" did not expose topicid`),
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: Topic %q did not expose topicid", schedulerName),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceSetDefaults,
			),
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
				WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceTopicFailed("TopicNotReady", fmt.Sprintf(`Topic "my-test-scheduler" mismatch: expected %q got "garbaaaaage"`, testTopicID)),
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: Topic %q mismatch: expected %q got "garbaaaaage"`, schedulerName, testTopicID),
		},
	}, {
		Name: "topic exists and the status topic is false",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceSetDefaults,
			),
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicFailed,
				WithTopicProjectID(testProject),
				WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceTopicFailed("TopicFailed", "test message"),
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: the status of Topic %q is False", schedulerName),
		},
	}, {
		Name: "topic exists and the status topic is unknown",
		Objects: []runtime.Object{
			NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithCloudSchedulerSourceSetDefaults,
			),
			NewTopic(schedulerName, testNS,
				WithTopicSpec(inteventsv1beta1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				WithTopicUnknown,
				WithTopicProjectID(testProject),
				WithTopicSetDefaults,
			),
			newSink(),
		},
		Key: testNS + "/" + schedulerName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudSchedulerSource(schedulerName, testNS,
				WithCloudSchedulerSourceSink(sinkGVK, sinkName),
				WithCloudSchedulerSourceLocation(location),
				WithCloudSchedulerSourceData(testData),
				WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
				WithInitCloudSchedulerSourceConditions,
				WithCloudSchedulerSourceTopicUnknown("", ""),
				WithCloudSchedulerSourceSetDefaults,
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: the status of Topic %q is Unknown", schedulerName),
		},
	},
		{
			Name: "topic exists and is ready, pullsubscription created",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceAnnotations(map[string]string{
						duckv1beta1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					}),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourceAnnotations(map[string]string{
						duckv1beta1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					}),
					WithCloudSchedulerSourceSetDefaults,
					WithCloudSchedulerSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
				),
			}},
			WantCreates: []runtime.Object{
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionLabels(map[string]string{
						"receive-adapter": receiveAdapterName,
						SourceLabelKey:    schedulerName}),
					WithPullSubscriptionAnnotations(map[string]string{
						"metrics-resource-group":          resourceGroup,
						duckv1beta1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					}),
					WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: PullSubscription %q has not yet been reconciled`, failedToPropagatePullSubscriptionStatusMsg, schedulerName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudScheduler),
					})),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: PullSubscription %q has not yet been reconciled`, failedToPropagatePullSubscriptionStatusMsg, schedulerName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS, WithPullSubscriptionFailed(),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudScheduler),
					})),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1beta1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: the status of PullSubscription %q is False`, failedToPropagatePullSubscriptionStatusMsg, schedulerName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS, WithPullSubscriptionUnknown(),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
						},
						AdapterType: string(converters.CloudScheduler),
					})),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionUnknown("", ""),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: the status of PullSubscription %q is Unknown`, failedToPropagatePullSubscriptionStatusMsg, schedulerName),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, create client fails",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
							Project: testProject,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					CreateClientErr: errors.New("create-client-induced-error"),
				},
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobNotReady(reconciledFailedReason, fmt.Sprintf("%s: %s", failedToReconcileJobMsg, "create-client-induced-error")),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Job failed with: create-client-induced-error"),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with non-grpc",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
							Project: testProject,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr: errors.New("get-job-induced-error"),
				},
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobNotReady(reconciledFailedReason, fmt.Sprintf("%s: %s", failedToReconcileJobMsg, "get-job-induced-error")),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Job failed with: get-job-induced-error"),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with grpc unknown error",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
							Project: testProject,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr: gstatus.Error(codes.Unknown, "get-job-induced-error"),
				},
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobNotReady(reconciledFailedReason, fmt.Sprintf("%s: rpc error: code = %s desc = %s", failedToReconcileJobMsg, codes.Unknown, "get-job-induced-error")),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledFailedReason, fmt.Sprintf("Reconcile Job failed with: rpc error: code = %s desc = %s", codes.Unknown, "get-job-induced-error")),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with grpc not found error, create job fails",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
							Project: testProject,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr:    gstatus.Error(codes.NotFound, "get-job-induced-error"),
					CreateJobErr: errors.New("create-job-induced-error"),
				},
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobNotReady(reconciledFailedReason, fmt.Sprintf("%s: %s", failedToReconcileJobMsg, "create-job-induced-error")),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Job failed with: create-job-induced-error"),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with grpc not found error, create job succeeds",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
							Project: testProject,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr: gstatus.Error(codes.NotFound, "get-job-induced-error"),
				},
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobReady(jobName),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `CloudSchedulerSource reconciled: "%s/%s"`, testNS, schedulerName),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, job exists",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						Project:           testProject,
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionSpec(inteventsv1beta1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1beta1.PubSubSpec{
							Secret: &secret,
							SourceSpec: duckv1.SourceSpec{
								Sink: newSinkDestination(),
							},
							Project: testProject,
						},
						AdapterType: string(converters.CloudScheduler),
					}),
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobReady(jobName),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", schedulerName),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `CloudSchedulerSource reconciled: "%s/%s"`, testNS, schedulerName),
			},
		}, {
			Name: "scheduler job fails to delete with no-grpc error",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceSubscriptionID(SubscriptionID),
					WithCloudSchedulerSourceJobReady(jobName),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceDeletionTimestamp,
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicSpec(inteventsv1beta1.TopicSpec{
						Topic:             testTopicID,
						PropagationPolicy: "CreateDelete",
						EnablePublisher:   &falseVal,
					}),
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key:               testNS + "/" + schedulerName,
			WantStatusUpdates: nil,
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					DeleteJobErr: errors.New("delete-job-induced-error"),
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, deleteJobFailed, "Failed to delete CloudSchedulerSource job: delete-job-induced-error"),
			},
		}, {
			Name: "scheduler job fails to delete with Unknown grpc error",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceJobReady(jobName),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceDeletionTimestamp,
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key:               testNS + "/" + schedulerName,
			WantStatusUpdates: nil,
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					DeleteJobErr: gstatus.Error(codes.Unknown, "delete-job-induced-error"),
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, deleteJobFailed, fmt.Sprintf("Failed to delete CloudSchedulerSource job: rpc error: code = %s desc = %s", codes.Unknown, "delete-job-induced-error")),
			},
		}, {
			Name: "scheduler successfully deleted with NotFound grpc error",
			Objects: []runtime.Object{
				NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceTopicReady(testTopicID, testProject),
					WithCloudSchedulerSourcePullSubscriptionReady,
					WithCloudSchedulerSourceJobReady(jobName),
					WithCloudSchedulerSourceSinkURI(schedulerSinkURL),
					WithCloudSchedulerSourceDeletionTimestamp,
					WithCloudSchedulerSourceSetDefaults,
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
					WithTopicSetDefaults,
				),
				NewPullSubscription(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudSchedulerSource(schedulerName, testNS,
					WithCloudSchedulerSourceProject(testProject),
					WithCloudSchedulerSourceSink(sinkGVK, sinkName),
					WithCloudSchedulerSourceLocation(location),
					WithCloudSchedulerSourceData(testData),
					WithCloudSchedulerSourceSchedule(onceAMinuteSchedule),
					WithInitCloudSchedulerSourceConditions,
					WithCloudSchedulerSourceJobReady(jobName),
					WithCloudSchedulerSourceTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", schedulerName)),
					WithCloudSchedulerSourcePullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", schedulerName)),
					WithCloudSchedulerSourceDeletionTimestamp,
					WithCloudSchedulerSourceSetDefaults,
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1beta1", Resource: "topics"}},
					Name: schedulerName,
				},
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1beta1", Resource: "pullsubscriptions"}},
					Name: schedulerName,
				},
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					DeleteJobErr: gstatus.Error(codes.NotFound, "delete-job-induced-error"),
				},
			},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase: intevents.NewPubSubBase(ctx,
				&intevents.PubSubBaseArgs{
					ControllerAgentName: controllerAgentName,
					ReceiveAdapterName:  receiveAdapterName,
					ReceiveAdapterType:  string(converters.CloudScheduler),
					ConfigWatcher:       cmw,
					// TODO remove after 0.16 cut.
					PubsubClientProvider: gpubsub.TestClientCreator(nil),
				}),
			Identity:        identity.NewIdentity(ctx, NoopIAMPolicyManager, NewGCPAuthTestStore(t, nil)),
			schedulerLister: listers.GetCloudSchedulerSourceLister(),
			createClientFn:  gscheduler.TestClientCreator(testData["scheduler"]),
		}
		return cloudschedulersource.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetCloudSchedulerSourceLister(), r.Recorder, r)
	}))

}
