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
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	schedulerv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	ops "github.com/google/knative-gcp/pkg/operations"
	operations "github.com/google/knative-gcp/pkg/operations/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	schedulerName = "my-test-scheduler"
	schedulerUID  = "test-scheduler-uid"
	sinkName      = "sink"

	testNS              = "testnamespace"
	testImage           = "scheduler-ops-image"
	topicUID            = schedulerName + "-abc-123"
	testProject         = "test-project-id"
	testTopicID         = "scheduler-" + schedulerUID
	testTopicURI        = "http://" + schedulerName + "-topic." + testNS + ".svc.cluster.local"
	location            = "us-central1"
	parentName          = "projects/" + testProject + "/locations/" + location
	jobName             = parentName + "/jobs/cre-scheduler-" + schedulerUID
	testData            = "mytestdata"
	onceAMinuteSchedule = "* * * * *"
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

	// Message for when the topic and pullsubscription with the above variables is not ready.
	topicNotReadyMsg            = "Topic testnamespace/my-test-scheduler not ready"
	pullSubscriptionNotReadyMsg = "PullSubscription testnamespace/my-test-scheduler not ready"
	jobNotCompletedMsg          = `Failed to create Scheduler Job: Job "my-test-scheduler" has not completed yet`
	jobFailedMsg                = `Failed to create Scheduler Job: Job "my-test-scheduler" failed to create or job failed`
	jobPodNotFoundMsg           = "Failed to create Scheduler Job: Pod not found"
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

	narSuccess := operations.JobActionResult{
		Result:    true,
		ProjectId: testProject,
		JobName:   jobName,
	}
	narFailure := operations.JobActionResult{
		Result:  false,
		Error:   "test induced failure",
		JobName: jobName,
	}

	successMsg, err := json.Marshal(narSuccess)
	if err != nil {
		t.Fatalf("Failed to marshal success JobActionResult: %s", err)
	}

	failureMsg, err := json.Marshal(narFailure)
	if err != nil {
		t.Fatalf("Failed to marshal failure JobActionResult: %s", err)
	}

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
			),
			newSink(),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithInitSchedulerConditions,
				WithSchedulerTopicNotReady("TopicNotReady", topicNotReadyMsg),
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
	}, {
		Name: "topic exists, topic not ready",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
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
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicNotReady("TopicNotReady", topicNotReadyMsg),
			),
		}},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
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
				WithInitSchedulerConditions,
				WithSchedulerTopicNotReady("TopicNotReady", "Topic testnamespace/my-test-scheduler did not expose projectid"),
				WithSchedulerFinalizers(finalizerName),
			),
		}},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
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
				WithInitSchedulerConditions,
				WithSchedulerTopicNotReady("TopicNotReady", "Topic testnamespace/my-test-scheduler did not expose topicid"),
				WithSchedulerFinalizers(finalizerName),
			),
		}},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
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
				WithInitSchedulerConditions,
				WithSchedulerTopicNotReady("TopicNotReady", `Topic testnamespace/my-test-scheduler topic mismatch expected "scheduler-test-scheduler-uid" got "garbaaaaage"`),
				WithSchedulerFinalizers(finalizerName),
			),
		}},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
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
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerFinalizers(finalizerName),
				WithSchedulerPullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
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
					"receive-adapter": "scheduler.events.cloud.google.com",
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": "schedulers.events.cloud.google.com",
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
			),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but is not ready",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
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
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job created",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
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
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithSchedulerData(testData),
				WithSchedulerSchedule(onceAMinuteSchedule),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", jobNotCompletedMsg),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
		WantCreates: []runtime.Object{
			newJob(NewScheduler(schedulerName, testNS), ops.ActionCreate),
		},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job not finished",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
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
			newJob(NewScheduler(schedulerName, testNS), ops.ActionCreate),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", jobNotCompletedMsg),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job failed",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
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
			newJobFinished(NewScheduler(schedulerName, testNS), ops.ActionCreate, false),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", jobFailedMsg),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job finished, no pods found",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
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
			newJobFinished(NewScheduler(schedulerName, testNS), ops.ActionCreate, true),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", jobPodNotFoundMsg),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job finished, no termination msg on pod",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
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
			newJobFinished(NewScheduler(schedulerName, testNS), ops.ActionCreate, true),
			newPod(""),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", `Failed to create Scheduler Job: did not find termination message for pod "test-pod"`),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job finished, invalid termination msg on pod",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
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
			newJobFinished(NewScheduler(schedulerName, testNS), ops.ActionCreate, true),
			newPod("invalid msg"),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", `Failed to create Scheduler Job: failed to unmarshal terminationmessage: "invalid msg" : "invalid character 'i' looking for beginning of value"`),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job finished, job creation failed",
		Objects: []runtime.Object{
			NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
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
			newJobFinished(NewScheduler(schedulerName, testNS), ops.ActionCreate, true),
			newPod(string(failureMsg)),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
				WithSchedulerSink(sinkGVK, sinkName),
				WithSchedulerLocation(location),
				WithSchedulerFinalizers(finalizerName),
				WithInitSchedulerConditions,
				WithSchedulerTopicReady(testTopicID, testProject),
				WithSchedulerPullSubscriptionReady(),
				WithSchedulerJobNotReady("JobNotReady", "Failed to create Scheduler Job: operation failed: test induced failure"),
				WithSchedulerSinkURI(schedulerSinkURL),
			),
		}},
	}, {
		Name: "topic and pullsubscription exist and ready, scheduler job finished, job created successfully",
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
			NewPullSubscriptionWithNoDefaults(schedulerName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
			newSink(),
			newJobFinished(NewScheduler(schedulerName, testNS), ops.ActionCreate, true),
			newPod(string(successMsg)),
		},
		Key:     testNS + "/" + schedulerName,
		WantErr: false,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewScheduler(schedulerName, testNS,
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
			),
		}},
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			SchedulerOpsImage: testImage,
			PubSubBase:        reconciler.NewPubSubBase(ctx, controllerAgentName, "scheduler.events.cloud.google.com", cmw),
			schedulerLister:   listers.GetSchedulerLister(),
			jobLister:         listers.GetJobLister(),
		}
	}))

}

func newJob(owner kmeta.OwnerRefable, action string) runtime.Object {
	if action == "create" {
		j, _ := operations.NewJobOps(operations.JobArgs{
			UID:      schedulerUID,
			JobName:  jobName,
			Image:    testImage,
			Action:   ops.ActionCreate,
			TopicID:  testTopicID,
			Secret:   secret,
			Owner:    owner,
			Data:     testData,
			Schedule: onceAMinuteSchedule,
		})
		return j
	}
	j, _ := operations.NewJobOps(operations.JobArgs{
		UID:     schedulerUID,
		Image:   testImage,
		JobName: jobName,
		Action:  ops.ActionDelete,
		Secret:  secret,
		Owner:   owner,
	})
	return j
}

func newJobFinished(owner kmeta.OwnerRefable, action string, success bool) runtime.Object {
	var job *batchv1.Job
	if action == "create" {
		job, _ = operations.NewJobOps(operations.JobArgs{
			UID:      schedulerUID,
			JobName:  jobName,
			Image:    testImage,
			Action:   ops.ActionCreate,
			TopicID:  testTopicID,
			Secret:   secret,
			Data:     testData,
			Owner:    owner,
			Schedule: onceAMinuteSchedule,
		})
	} else {
		job, _ = operations.NewJobOps(operations.JobArgs{
			UID:    schedulerUID,
			Image:  testImage,
			Action: ops.ActionDelete,
			Secret: secret,
			Owner:  owner,
		})
	}

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

func newPod(msg string) runtime.Object {
	labels := map[string]string{
		"resource-uid": schedulerUID,
		"action":       "create",
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: testNS,
			Labels:    labels,
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: msg,
						},
					},
				},
			},
		},
	}
}
