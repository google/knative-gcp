/*
Copyright 2020 Google LLC

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

package build

import (
	"context"
	"errors"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudbuildsource"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	reconcilerpubsub "github.com/google/knative-gcp/pkg/reconciler/pubsub"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	buildName = "my-test-build"
	buildUID  = "test-build-uid"
	sinkName  = "sink"

	testNS                                     = "testnamespace"
	testTopicID                                = v1alpha1.DefaultTopic
	generation                                 = 1
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
)

var (
	trueVal  = true
	falseVal = false

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = apis.HTTP(sinkDNS)

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

	gServiceAccount = "test123@test123.iam.gserviceaccount.com"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

// Returns an ownerref for the test CloudBuildSource object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "CloudBuildSource",
		Name:               buildName,
		UID:                buildUID,
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

// TODO add a unit test for successfully creating a k8s service account, after issue https://github.com/google/knative-gcp/issues/657 gets solved.
func TestAllCases(t *testing.T) {
	attempts := 0
	pubsubSinkURL := sinkURI

	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "pullsubscription created",
			Objects: []runtime.Object{
				NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
				),
				newSink(),
			},
			Key: testNS + "/" + buildName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceStatusObservedGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithInitCloudBuildSourceConditions,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", "PullSubscription has not yet been reconciled"),
				),
			}},
			WantCreates: []runtime.Object{
				NewPullSubscriptionWithNoDefaults(buildName, testNS,
					WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
						Topic: testTopicID,
						PubSubSpec: duckv1alpha1.PubSubSpec{
							Secret: &secret,
						},
					}),
					WithPullSubscriptionSink(sinkGVK, sinkName),
					WithPullSubscriptionLabels(map[string]string{
						"receive-adapter":                     receiveAdapterName,
						"events.cloud.google.com/source-name": buildName,
					}),
					WithPullSubscriptionAnnotations(map[string]string{
						"metrics-resource-group": resourceGroup,
					}),
					WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{ownerRef()}),
				),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, buildName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", buildName),
				Eventf(corev1.EventTypeWarning, reconcilerpubsub.PullSubscriptionStatusPropagateFailedReason, "%s: PullSubscription %q has not yet been reconciled", failedToPropagatePullSubscriptionStatusMsg, buildName),
			},
		}, {
			Name: "pullsubscription exists and the status is false",
			Objects: []runtime.Object{
				NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
				),
				NewPullSubscriptionWithNoDefaults(buildName, testNS,
					WithPullSubscriptionReadyStatus(corev1.ConditionFalse, "PullSubscriptionFalse", "status false test message")),
				newSink(),
			},
			Key: testNS + "/" + buildName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceStatusObservedGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithInitCloudBuildSourceConditions,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourcePullSubscriptionFailed("PullSubscriptionFalse", "status false test message"),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, buildName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", buildName),
				Eventf(corev1.EventTypeWarning, reconcilerpubsub.PullSubscriptionStatusPropagateFailedReason, "%s: the status of PullSubscription %q is False", failedToPropagatePullSubscriptionStatusMsg, buildName),
			},
		}, {
			Name: "pullsubscription exists and the status is unknown",
			Objects: []runtime.Object{
				NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
				),
				NewPullSubscriptionWithNoDefaults(buildName, testNS,
					WithPullSubscriptionReadyStatus(corev1.ConditionUnknown, "PullSubscriptionUnknown", "status unknown test message")),
				newSink(),
			},
			Key: testNS + "/" + buildName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceStatusObservedGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithInitCloudBuildSourceConditions,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourcePullSubscriptionUnknown("PullSubscriptionUnknown", "status unknown test message"),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, buildName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", buildName),
				Eventf(corev1.EventTypeWarning, reconcilerpubsub.PullSubscriptionStatusPropagateFailedReason, "%s: the status of PullSubscription %q is Unknown", failedToPropagatePullSubscriptionStatusMsg, buildName),
			},
		}, {
			Name: "pullsubscription exists and ready, with retry",
			Objects: []runtime.Object{
				NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
				),
				NewPullSubscriptionWithNoDefaults(buildName, testNS,
					WithPullSubscriptionReady(sinkURI),
					WithPullSubscriptionReadyStatus(corev1.ConditionTrue, "PullSubscriptionNoReady", ""),
				),
				newSink(),
			},
			Key: testNS + "/" + buildName,
			WithReactors: []clientgotesting.ReactionFunc{
				func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					if attempts != 0 || !action.Matches("update", "cloudbuildsources") {
						return false, nil, nil
					}
					attempts++
					return true, nil, apierrs.NewConflict(v1alpha1.Resource("foo"), "bar", errors.New("foo"))
				},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceStatusObservedGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithInitCloudBuildSourceConditions,
					WithCloudBuildSourcePullSubscriptionReady(),
					WithCloudBuildSourceSinkURI(pubsubSinkURL),
				),
			}, {
				Object: NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceStatusObservedGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithInitCloudBuildSourceConditions,
					WithCloudBuildSourcePullSubscriptionReady(),
					WithCloudBuildSourceSinkURI(pubsubSinkURL),
					WithCloudBuildSourceFinalizers("cloudbuildsources.events.cloud.google.com"),
				),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, buildName, true),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", buildName),
				Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `CloudBuildSource reconciled: "%s/%s"`, testNS, buildName),
			},
		}, {
			Name: "pullsubscription deleted with getting k8s service account error",
			Objects: []runtime.Object{
				NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithCloudBuildSourceDeletionTimestamp,
					WithCloudBuildSourceGCPServiceAccount(gServiceAccount),
					WithCloudBuildSourceServiceAccountName("test123"),
				),
				newSink(),
			},
			Key: testNS + "/" + buildName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewCloudBuildSource(buildName, testNS,
					WithCloudBuildSourceObjectMetaGeneration(generation),
					WithCloudBuildSourceTopic(testTopicID),
					WithCloudBuildSourceSink(sinkGVK, sinkName),
					WithCloudBuildSourceDeletionTimestamp,
					WithCloudBuildSourceWorkloadIdentityFailed("WorkloadIdentityDeleteFailed", `serviceaccounts "test123" not found`),
					WithCloudBuildSourceGCPServiceAccount(gServiceAccount),
					WithCloudBuildSourceServiceAccountName("test123"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "WorkloadIdentityDeleteFailed", `Failed to delete CloudBuildSource workload identity: getting k8s service account failed with: serviceaccounts "test123" not found`),
			},
		}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, _ map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			PubSubBase:             pubsub.NewPubSubBase(ctx, controllerAgentName, receiveAdapterName, cmw),
			Identity:               identity.NewIdentity(ctx),
			buildLister:            listers.GetCloudBuildSourceLister(),
			pullsubscriptionLister: listers.GetPullSubscriptionLister(),
			serviceAccountLister:   listers.GetServiceAccountLister(),
		}
		return cloudbuildsource.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetCloudBuildSourceLister(), r.Recorder, r)
	}))

}
