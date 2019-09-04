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

	//	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	//	"knative.dev/pkg/kmeta"

	//	batchv1 "k8s.io/api/batch/v1"
	//	corev1 "k8s.io/api/core/v1"
	//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	//	"k8s.io/client-go/kubernetes/scheme"
	//	clientgotesting "k8s.io/client-go/testing"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	//	"knative.dev/pkg/configmap"
	//	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	//	schedulerv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	//	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	//fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/pubsub/v1alpha1/fake"
	fakePubsubClient "github.com/google/knative-gcp/pkg/client/clientset/versioned/fake"
	//	ops "github.com/google/knative-gcp/pkg/operations"
	//	operations "github.com/google/knative-gcp/pkg/operations/scheduler"
	//	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	//	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS = "test-namespace"
	name   = "obj-name"
)

func TestAllCases(t *testing.T) {

	testCases := []struct {
		name string
	}{{
		name: "foobar",
	}, {
		name: "foobar",
	}}

	/*
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
						"receive-adapter": "scheduler.events.cloud.run",
					}),
					WithTopicOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, schedulerName, true),
			},
		}}

	*/
	defer logtesting.ClearAll()

	for _, tc := range testCases {
		cs := fakePubsubClient.NewSimpleClientset()
		//		psClient := cs.PubsubV1alpha1()
		ps := &PubSubBase{
			pubsubClient: cs,
		}
		spec := &duckv1alpha1.PubSubSpec{}
		status := &duckv1alpha1.PubSubStatus{}
		condSet := &apis.ConditionSet{}
		_, _, err := ps.ReconcilePubSub(context.Background(), testNS, name, spec, status, condSet, nil, "topic")
		if err != nil {
			t.Errorf("%q Failed: %s", tc.name, err)
		}
	}

	/*
		table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
			return &Reconciler{
				SchedulerOpsImage: testImage,
				PubSubBase:        reconciler.NewPubSubBase(ctx, controllerAgentName, "scheduler.events.cloud.run", cmw),
				schedulerLister:   listers.GetSchedulerLister(),
				jobLister:         listers.GetJobLister(),
			}
		}))
	*/

}
