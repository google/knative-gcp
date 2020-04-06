/*
Copyright 2019 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auditlogs

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudauditlogssource"
	testiam "github.com/google/knative-gcp/pkg/gclient/iam/testing"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
	glogadmintesting "github.com/google/knative-gcp/pkg/gclient/logging/logadmin/testing"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	sourceName        = "test-cloudauditlogssource"
	sourceUID         = "test-cloudauditlogssource-uid"
	testNS            = "testnamespace"
	testProject       = "test-project-id"
	testTopicID       = "cloudauditlogssource-" + sourceUID
	testTopicResource = "pubsub.googleapis.com/projects/" + testProject + "/topics/" + testTopicID
	testTopicURI      = "http://" + sourceName + "-topic." + testNS + ".svc.cluster.local"
	testSinkID        = "sink-" + sourceUID

	testServiceName = "test-service"
	testMethodName  = "test-method"
	testFilter      = `protoPayload.methodName="test-method" AND protoPayload.serviceName="test-service" AND protoPayload."@type"="type.googleapis.com/google.cloud.audit.AuditLog"`

	sinkName = "sink"
	sinkDNS  = sinkName + ".mynamespace.svc.cluster.local"

	topicNotReadyMsg                           = `Topic "test-cloudauditlogssource" not ready`
	pullSubscriptionNotReadyMsg                = `PullSubscription "test-cloudauditlogssource" not ready`
	failedToReconcileTopicMsg                  = `Topic has not yet been reconciled`
	failedToReconcilePullSubscriptionMsg       = `PullSubscription has not yet been reconciled`
	failedToCreateSinkMsg                      = `failed to ensure creation of logging sink`
	failedToSetPermissionsMsg                  = `failed to ensure sink has pubsub.publisher permission on source topic`
	failedToDeleteSinkMsg                      = `Failed to delete Stackdriver sink`
	failedToPropagatePullSubscriptionStatusMsg = `Failed to propagate PullSubscription status`
)

var (
	trueVal  = true
	falseVal = false

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

	sinkURI = apis.HTTP(sinkDNS)
)

func sourceOwnerRef(name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "CloudAuditLogsSource",
		Name:               name,
		UID:                uid,
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

// TODO add a unit test for successfully creating a k8s service account, after issue https://github.com/google/knative-gcp/issues/657 gets solved.
func TestAllCases(t *testing.T) {
	calSinkURL := sinkURI

	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "topic created, not yet been reconciled",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName)),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg)),
		}},
		WantCreates: []runtime.Object{
			NewTopic(sourceName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             "cloudauditlogssource-" + sourceUID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": sourceName,
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{sourceOwnerRef(sourceName, sourceUID)}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: Topic %q has not yet been reconciled", sourceName),
		},
	}, {
		Name: "topic exists, topic has not yet been reconciled",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
			),
			NewTopic(sourceName, testNS,
				WithTopicTopicID(testTopicID),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName)),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: the status of Topic %q is Unknown", sourceName),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicFailed("TopicNotReady", `Topic "test-cloudauditlogssource" did not expose projectid`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: Topic %q did not expose projectid", sourceName),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicFailed("TopicNotReady", `Topic "test-cloudauditlogssource" did not expose topicid`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: Topic %q did not expose topicid", sourceName),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicFailed("TopicNotReady", `Topic "test-cloudauditlogssource" mismatch: expected "cloudauditlogssource-test-cloudauditlogssource-uid" got "garbaaaaage"`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: Topic %q mismatch: expected "cloudauditlogssource-test-cloudauditlogssource-uid" got "garbaaaaage"`, sourceName),
		},
	}, {
		Name: "topic exists and the status of topic is false",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
			),
			NewTopic(sourceName, testNS,
				WithTopicFailed(),
				WithTopicTopicID(testTopicID),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicFailed("PublisherStatus", "Publisher has no Ready type status")),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: the status of Topic %q is False", sourceName),
		},
	}, {
		Name: "topic exists and the status of topic is unknown",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
			),
			NewTopic(sourceName, testNS,
				WithTopicUnknown(),
				WithTopicTopicID(testTopicID),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicUnknown("", "")),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: the status of Topic %q is Unknown", sourceName),
		},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret: &secret,
					},
					AdapterType: converters.CloudAuditLogsConverter,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": sourceName,
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{sourceOwnerRef(sourceName, sourceUID)}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: PullSubscription %q has not yet been reconciled`, failedToPropagatePullSubscriptionStatusMsg, sourceName),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but has not yet been reconciled",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: PullSubscription %q has not yet been reconciled`, failedToPropagatePullSubscriptionStatusMsg, sourceName),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is false",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS, WithPullSubscriptionFailed()),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, `Reconcile PubSub failed with: %s: the status of PullSubscription %q is False`, failedToPropagatePullSubscriptionStatusMsg, sourceName),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists and the status of pullsubscription is unknown",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS, WithPullSubscriptionUnknown()),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionUnknown("", ""),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: %s: the status of PullSubscription %q is Unknown", failedToPropagatePullSubscriptionStatusMsg, sourceName),
		},
	}, {
		Name: "logging client create fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"logadmin": glogadmintesting.TestClientConfiguration{
				CreateClientErr: errors.New("create-client-induced-error"),
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "get sink fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"logadmin": glogadmintesting.TestClientConfiguration{
				SinkErr: errors.New("create-client-induced-error"),
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "create sink fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"logadmin": glogadmintesting.TestClientConfiguration{
				CreateSinkErr: errors.New("create-client-induced-error"),
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, pubsub client create fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"pubsub": gpubsub.TestClientData{
				CreateClientErr: errors.New("create-client-induced-error"),
			},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, get pubsub IAM policy fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceMethodName(testMethodName)),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"pubsub": gpubsub.TestClientData{
				HandleData: testiam.TestHandleData{
					PolicyErr: errors.New("create-client-induced-error"),
				},
			},
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: {
					ID:          testSinkID,
					Filter:      testFilter,
					Destination: testTopicResource,
				}},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, set pubsub IAM policy fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceMethodName(testMethodName)),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"pubsub": gpubsub.TestClientData{
				HandleData: testiam.TestHandleData{
					SetPolicyErr: errors.New("create-client-induced-error"),
				},
			},
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: {
					ID:          testSinkID,
					Filter:      testFilter,
					Destination: testTopicResource,
				}},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceMethodName(testMethodName)),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: {
					ID:          testSinkID,
					Filter:      testFilter,
					Destination: testTopicResource,
				}},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `CloudAuditLogsSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceSinkID(testSinkID),
			),
		}},
	}, {
		Name: "sink exists",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"existingSinks": []logadmin.Sink{{
				ID:          testSinkID,
				Filter:      testFilter,
				Destination: testTopicResource,
			}},
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: {
					ID:          testSinkID,
					Filter:      testFilter,
					Destination: testTopicResource,
				}},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `CloudAuditLogsSource reconciled: "%s/%s"`, testNS, sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceSinkID(testSinkID),
			),
		}},
	}, {
		Name: "sink delete fails",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceSinkID(testSinkID),
				WithCloudAuditLogsSourceDeletionTimestamp,
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"existingSinks": []logadmin.Sink{{
				ID:          testSinkID,
				Filter:      testFilter,
				Destination: testTopicResource,
			}},
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: {
					ID:          testSinkID,
					Filter:      testFilter,
					Destination: testTopicResource,
				}},
			"logadmin": glogadmintesting.TestClientConfiguration{
				DeleteSinkErr: errors.New("delete-sink-induced-error"),
			},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, deleteSinkFailed, "Failed to delete Stackdriver sink: delete-sink-induced-error"),
		},
		WantStatusUpdates: nil,
	}, {
		Name: "sink delete succeeds",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceSinkID(testSinkID),
				WithCloudAuditLogsSourceDeletionTimestamp,
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"existingSinks": []logadmin.Sink{{
				ID:          testSinkID,
				Filter:      testFilter,
				Destination: testTopicResource,
			}},
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: nil,
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", sourceName)),
				WithCloudAuditLogsSourcePullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", sourceName)),
				WithCloudAuditLogsSourceDeletionTimestamp,
			),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}},
				Name: sourceName,
			},
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}},
				Name: sourceName,
			},
		},
	}, {
		Name: "delete succeeds, sink does not exist",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceProjectID(testProject),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceTopicReady(testTopicID),
				WithCloudAuditLogsSourcePullSubscriptionReady(),
				WithCloudAuditLogsSourceSinkURI(calSinkURL),
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceSinkID(testSinkID),
				WithCloudAuditLogsSourceDeletionTimestamp,
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: nil,
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceSinkReady(),
				WithCloudAuditLogsSourceTopicFailed("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", sourceName)),
				WithCloudAuditLogsSourcePullSubscriptionFailed("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", sourceName)),
				WithCloudAuditLogsSourceDeletionTimestamp,
			),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "topics"}},
				Name: sourceName,
			},
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "pubsub.cloud.google.com", Version: "v1alpha1", Resource: "pullsubscriptions"}},
				Name: sourceName,
			},
		},
	}, {
		Name: "delete failed with getting k8s service account error",
		Objects: []runtime.Object{
			NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceGCPServiceAccount(gServiceAccount),
				WithCloudAuditLogsSourceDeletionTimestamp,
				WithCloudAuditLogsSourceServiceAccountName("test123"),
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewCloudAuditLogsSource(sourceName, testNS,
				WithCloudAuditLogsSourceMethodName(testMethodName),
				WithCloudAuditLogsSourceServiceName(testServiceName),
				WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				WithInitCloudAuditLogsSourceConditions,
				WithCloudAuditLogsSourceGCPServiceAccount(gServiceAccount),
				WithCloudAuditLogsSourceWorkloadIdentityFailed("WorkloadIdentityDeleteFailed", `serviceaccounts "test123" not found`),
				WithCloudAuditLogsSourceDeletionTimestamp,
				WithCloudAuditLogsSourceServiceAccountName("test123"),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "WorkloadIdentityDeleteFailed", `Failed to delete CloudAuditLogsSource workload identity: getting k8s service account failed with: serviceaccounts "test123" not found`),
		},
	}}

	defer logtesting.ClearAll()
	for _, tt := range table {
		t.Run(tt.Name, func(t *testing.T) {
			logadminClientProvider := glogadmintesting.TestClientCreator(tt.OtherTestData["logadmin"])
			if existingSinks := tt.OtherTestData["existingSinks"]; existingSinks != nil {
				createSinks(t, logadminClientProvider, existingSinks.([]logadmin.Sink))
			}
			tt.Test(t, MakeFactory(
				func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
					r := &Reconciler{
						PubSubBase:             pubsub.NewPubSubBaseWithAdapter(ctx, controllerAgentName, receiveAdapterName, converters.CloudAuditLogsConverter, cmw),
						Identity:               identity.NewIdentity(ctx),
						auditLogsSourceLister:  listers.GetCloudAuditLogsSourceLister(),
						logadminClientProvider: logadminClientProvider,
						pubsubClientProvider:   gpubsub.TestClientCreator(testData["pubsub"]),
						serviceAccountLister:   listers.GetServiceAccountLister(),
					}
					return cloudauditlogssource.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetCloudAuditLogsSourceLister(), r.Recorder, r)
				}))
			if expectedSinks := tt.OtherTestData["expectedSinks"]; expectedSinks != nil {
				expectSinks(t, logadminClientProvider, expectedSinks.(map[string]*logadmin.Sink))
			}
		})
	}
}

func createSinks(t *testing.T, clientProvider glogadmin.CreateFn, sinks []logadmin.Sink) {
	logadminClient, err := clientProvider(context.Background(), testProject)
	if err != nil {
		t.Fatalf("failed to create logadmin client during setup: %s", err)
	}
	for _, sink := range sinks {
		logadminClient.CreateSink(context.Background(), &sink)
	}
}

func expectSinks(t *testing.T, clientProvider glogadmin.CreateFn, sinks map[string]*logadmin.Sink) {
	logadminClient, err := clientProvider(context.Background(), testProject)
	if err != nil {
		t.Fatalf("failed to create logadmin client during verification: %s", err)
	}
	for sinkID, sink := range sinks {
		actual, err := logadminClient.Sink(context.Background(), sinkID)
		if err != nil && !(status.Code(err) == codes.NotFound && sink == nil) {
			t.Errorf("failed to get expected sink %s: %v", sinkID, err)
		}
		if diff := cmp.Diff(sink, actual, cmpopts.IgnoreFields(logadmin.Sink{}, "WriterIdentity")); diff != "" {
			t.Log("Unexpected difference in sink:")
			t.Log(diff)
			t.Fail()
		}
	}
}
