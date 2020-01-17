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

	schedulerv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	gscheduler "github.com/google/knative-gcp/pkg/gclient/scheduler/testing"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	schedulerName = "my-test-scheduler"
	schedulerUID  = "test-scheduler-uid"
	sinkName      = "sink"

	testNS              = "testnamespace"
	testImage           = "scheduler-ops-image"
	testProject         = "test-project-id"
	testTopicID         = "scheduler-" + schedulerUID
	testTopicURI        = "http://" + schedulerName + "-topic." + testNS + ".svc.cluster.local"
	location            = "us-central1"
	parentName          = "projects/" + testProject + "/locations/" + location
	jobName             = parentName + "/jobs/cre-scheduler-" + schedulerUID
	testData            = "mytestdata"
	onceAMinuteSchedule = "* * * * *"

	// Message for when the topic and pullsubscription with the above variables are not ready.
	failedToReconcileTopicMsg            = `Topic has not yet been reconciled`
	failedToReconcilePullSubscriptionMsg = `PullSubscription has not yet been reconciled`
	failedToReconcileJobMsg              = `Failed to reconcile Scheduler job`
	failedToDeleteJobMsg                 = `Failed to delete Scheduler job`
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
			Name: "google-cloud-key",
		},
		Key: "key.json",
	}
)

func init() {
	// Add types to scheme
	_ = schedulerv1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test Scheduler object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "Scheduler",
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
		fname = fmt.Sprintf("%q", finalizerName)
	}
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
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

// turn string into URL or terminate with t.Fatalf
func sinkURL(t *testing.T, url string) *apis.URL {
	u, err := apis.ParseURL(url)
	if err != nil {
		t.Fatalf("Failed to parse url %q", url)
	}
	return u
}

func TestAllCases(t *testing.T) {
	schedulerSinkURL := sinkURL(t, sinkURI)

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
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithInitSchedulerConditions,
				WithSchedulerTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewTopic(schedulerName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter": "scheduler.events.cloud.google.com",
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, schedulerName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated Scheduler %q finalizers", schedulerName),
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q has not yet been reconciled", schedulerName),
		},
	}, {
		Name: "topic exists, topic has not yet been reconciled",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
			),
			NewTopic(schedulerName, testNS,
				WithTopicTopicID(testTopicID),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q has not yet been reconciled", schedulerName),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
			),
			NewTopic(schedulerName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithInitSchedulerConditions,
				WithSchedulerTopicFailed("TopicNotReady", `Topic "my-test-scheduler" did not expose projectid`),
				WithSchedulerFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose projectid", schedulerName),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
			),
			NewTopic(schedulerName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithInitSchedulerConditions,
				WithSchedulerTopicFailed("TopicNotReady", `Topic "my-test-scheduler" did not expose topicid`),
				WithSchedulerFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose topicid", schedulerName),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
			),
			NewTopic(schedulerName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithInitSchedulerConditions,
				WithSchedulerTopicFailed("TopicNotReady", `Topic "my-test-scheduler" mismatch: expected "scheduler-test-scheduler-uid" got "garbaaaaage"`),
				WithSchedulerFinalizers(finalizerName),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Topic %q mismatch: expected "scheduler-test-scheduler-uid" got "garbaaaaage"`, schedulerName),
		},
	}, {
		Name: "topic exists and the status topic is false",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
			),
			NewTopic(schedulerName, testNS,
				WithTopicFailed(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicFailed("PublisherStatus", "Publisher has no Ready type status"),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "the status of Topic %q is False", schedulerName),
		},
	}, {
		Name: "topic exists and the status topic is unknown",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
			),
			NewTopic(schedulerName, testNS,
				WithTopicUnknown(),
				WithTopicProjectID(testProject),
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicUnknown("", ""),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "the status of Topic %q is Unknown", schedulerName),
		},
	},
		{
			Name: "topic exists and is ready, pullsubscription created",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				newSink(),
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerPullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
				),
			}},
			WantCreates: []runtime.Object{
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
						Topic:  testTopicID,
						Secret: &secret,
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionLabels(map[string]string{
						"receive-adapter": receiveAdapterName,
					}),
					WithPullSubscriptionAnnotations(map[string]string{
						"metrics-resource-group": resourceGroup,
					}),
					WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q has not yet been reconciled", schedulerName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS),
				newSink(),
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q has not yet been reconciled", schedulerName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS, WithPullSubscriptionFailed()),
				newSink(),
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "the status of PullSubscription %q is False", schedulerName),
			},
		}, {
			Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS, WithPullSubscriptionUnknown()),
				newSink(),
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithSchedulerFinalizers(finalizerName),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionUnknown("", ""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "the status of PullSubscription %q is Unknown", schedulerName),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, create client fails",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					CreateClientErr: errors.New("create-client-induced-error"),
				},
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobNotReady("JobReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileJobMsg, "create-client-induced-error")),
					WithSchedulerSinkURI(schedulerSinkURL)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with non-grpc",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr: errors.New("get-job-induced-error"),
				},
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobNotReady("JobReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileJobMsg, "get-job-induced-error")),
					WithSchedulerSinkURI(schedulerSinkURL)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "get-job-induced-error"),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with grpc unknown error",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr: gstatus.Error(codes.Unknown, "get-job-induced-error"),
				},
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobNotReady("JobReconcileFailed", fmt.Sprintf("%s: rpc error: code = %s desc = %s", failedToReconcileJobMsg, codes.Unknown, "get-job-induced-error")),
					WithSchedulerSinkURI(schedulerSinkURL)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("rpc error: code = %s desc = %s", codes.Unknown, "get-job-induced-error")),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with grpc not found error, create job fails",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					GetJobErr:    gstatus.Error(codes.NotFound, "get-job-induced-error"),
					CreateJobErr: errors.New("create-job-induced-error"),
				},
			},
			Key:     testNS + "/" + schedulerName,
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobNotReady("JobReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileJobMsg, "create-job-induced-error")),
					WithSchedulerSinkURI(schedulerSinkURL)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "create-job-induced-error"),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, get job fails with grpc not found error, create job succeeds",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
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
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobReady(jobName),
					WithSchedulerSinkURI(schedulerSinkURL)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ReadinessChanged", "Scheduler %q became ready", schedulerName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Scheduler %q", schedulerName),
			},
		}, {
			Name: "topic and pullsubscription exist and ready, job exists",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobReady(jobName),
					WithSchedulerSinkURI(schedulerSinkURL)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "ReadinessChanged", "Scheduler %q became ready", schedulerName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Scheduler %q", schedulerName),
			},
		}, {
			Name: "scheduler job fails to delete with no-grpc error",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobReady(jobName),
					WithSchedulerSinkURI(schedulerSinkURL),
					WithSchedulerDeletionTimestamp,
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobName(jobName),
					WithSchedulerJobNotReady("JobDeleteFailed", fmt.Sprintf("%s: %s", failedToDeleteJobMsg, "delete-job-induced-error")),
					WithSchedulerSinkURI(schedulerSinkURL),
					WithSchedulerDeletionTimestamp,
				),
			}},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					DeleteJobErr: errors.New("delete-job-induced-error"),
				},
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "delete-job-induced-error"),
			},
		}, {
			Name: "scheduler job fails to delete with Unknown grpc error",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobReady(jobName),
					WithSchedulerSinkURI(schedulerSinkURL),
					WithSchedulerDeletionTimestamp,
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobName(jobName),
					WithSchedulerJobNotReady("JobDeleteFailed", fmt.Sprintf("%s: rpc error: code = %s desc = %s", failedToDeleteJobMsg, codes.Unknown, "delete-job-induced-error")),
					WithSchedulerSinkURI(schedulerSinkURL),
					WithSchedulerDeletionTimestamp,
				),
			}},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					DeleteJobErr: gstatus.Error(codes.Unknown, "delete-job-induced-error"),
				},
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("rpc error: code = %s desc = %s", codes.Unknown, "delete-job-induced-error")),
			},
		}, {
			Name: "scheduler successfully deleted with NotFound grpc error",
			Objects: []runtime.Object{
				NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerTopicReady(testTopicID, testProject),
					WithSchedulerPullSubscriptionReady(),
					WithSchedulerJobReady(jobName),
					WithSchedulerSinkURI(schedulerSinkURL),
					WithSchedulerDeletionTimestamp,
				),
				NewTopic(schedulerName, testNS,
					WithTopicReady(testTopicID),
					WithTopicAddress(testTopicURI),
					WithTopicProjectID(testProject),
				),
				NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
					WithPullSubscriptionReady(sinkURI),
				),
				newSink(),
			},
			Key: testNS + "/" + schedulerName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewScheduler(schedulerName, testNS,
					WithSchedulerProject(testProject),
					WithSchedulerSink(sinkGVK, sinkName),
					WithSchedulerLocation(location),
					WithSchedulerFinalizers(finalizerName),
					WithSchedulerData(testData),
					WithSchedulerSchedule(onceAMinuteSchedule),
					WithInitSchedulerConditions,
					WithSchedulerJobNotReady("JobDeleted", fmt.Sprintf("Successfully deleted Scheduler job: %s", jobName)),
					WithSchedulerTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", schedulerName)),
					WithSchedulerPullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", schedulerName)),
					WithSchedulerDeletionTimestamp,
				),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}},
					Name: schedulerName,
				},
				{ActionImpl: clientgotesting.ActionImpl{
					Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}},
					Name: schedulerName,
				},
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, false),
			},
			OtherTestData: map[string]interface{}{
				"scheduler": gscheduler.TestClientData{
					DeleteJobErr: gstatus.Error(codes.NotFound, "delete-job-induced-error"),
				},
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Scheduler %q finalizers", schedulerName),
				Eventf(corev1.EventTypeNormal, "Updated", "Updated Scheduler %q", schedulerName),
			},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		return &Reconciler{
			PubSubBase:      pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			schedulerLister: listers.GetSchedulerLister(),
			createClientFn:  gscheduler.TestClientCreator(testData["scheduler"]),
		}
	}))

}
