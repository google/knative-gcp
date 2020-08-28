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

	v1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"

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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/duck"
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	inteventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudauditlogssource"
	testiam "github.com/google/knative-gcp/pkg/gclient/iam/testing"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
	glogadmintesting "github.com/google/knative-gcp/pkg/gclient/logging/logadmin/testing"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	sourceName   = "test-cal"
	sourceUID    = "test-cal-uid"
	testNS       = "testnamespace"
	testProject  = "test-project-id"
	testTopicURI = "http://" + sourceName + "-topic." + testNS + ".svc.cluster.local"

	testServiceName = "test-service"
	testMethodName  = "test-method"
	testFilter      = `protoPayload.methodName="test-method" AND protoPayload.serviceName="test-service" AND protoPayload."@type"="type.googleapis.com/google.cloud.audit.AuditLog"`

	sinkName = "sink"
	sinkDNS  = sinkName + ".mynamespace.svc.cluster.local"

	topicNotReadyMsg                           = `Topic "test-cal" not ready`
	pullSubscriptionNotReadyMsg                = `PullSubscription "test-cal" not ready`
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
		Version: "v1",
		Kind:    "Sink",
	}

	testTopicID       = fmt.Sprintf("cre-src_%s_%s_%s", testNS, sourceName, sourceUID)
	testSinkID        = fmt.Sprintf("cre-src_%s_%s_%s", testNS, sourceName, sourceUID)
	testTopicResource = fmt.Sprintf("pubsub.googleapis.com/projects/%s/topics/%s", testProject, testTopicID)

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
		APIVersion:         "events.cloud.google.com/v1",
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

func newSinkDestination() duckv1.Destination {
	return duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "testing.cloud.google.com/v1",
			Kind:       "Sink",
			Name:       sinkName,
		},
	}
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceTopicUnknown("TopicNotConfigured", failedToReconcileTopicMsg),
				v1.WithCloudAuditLogsSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		}},
		WantCreates: []runtime.Object{
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": sourceName,
				}),
				v1.WithTopicOwnerReferences([]metav1.OwnerReference{sourceOwnerRef(sourceName, sourceUID)}),
				v1.WithTopicAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithTopicSetDefaults,
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicUnknown,
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceTopicFailed("TopicNotReady", fmt.Sprintf(`Topic %q did not expose projectid`, sourceName)),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(""),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceTopicFailed("TopicNotReady", fmt.Sprintf("Topic %q did not expose topicid", sourceName)),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady("garbaaaaage"),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceTopicFailed("TopicNotReady", fmt.Sprintf(`Topic %q mismatch: expected %q got "garbaaaaage"`, sourceName, testTopicID)),
			),
		}},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, reconciledPubSubFailedReason, fmt.Sprintf(`Reconcile PubSub failed with: Topic %q mismatch: expected %q got "garbaaaaage"`, sourceName, testTopicID)),
		},
	}, {
		Name: "topic exists and the status of topic is false",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicFailed,
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicFailed("TopicFailed", "test message"),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicUnknown,
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicUnknown("", ""),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourceAnnotations(map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
			),
		}},
		WantCreates: []runtime.Object{
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
					},
					AdapterType: string(converters.CloudAuditLogs),
				}),
				v1.WithPullSubscriptionSink(sinkGVK, sinkName),
				v1.WithPullSubscriptionLabels(map[string]string{
					"receive-adapter":                     receiveAdapterName,
					"events.cloud.google.com/source-name": sourceName,
				}),
				v1.WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group":   resourceGroup,
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				}),
				v1.WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{sourceOwnerRef(sourceName, sourceUID)}),
				v1.WithPullSubscriptionDefaultGCPAuth,
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourcePullSubscriptionUnknown("PullSubscriptionNotConfigured", failedToReconcilePullSubscriptionMsg),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS, v1.WithPullSubscriptionFailed(),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourcePullSubscriptionFailed("InvalidSink", `failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`),
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS, v1.WithPullSubscriptionUnknown(),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
		},
		Key: testNS + "/" + sourceName,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionUnknown("", ""),
				v1.WithCloudAuditLogsSourceSetDefaults,
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
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "get sink fails",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "create sink fails",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, pubsub client create fails",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, get pubsub IAM policy fails",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, set pubsub IAM policy fails",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSetDefaults,
				v1.WithCloudAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSinkReady,
				v1.WithCloudAuditLogsSourceSinkID(testSinkID),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		}},
	}, {
		Name: "sink exists",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicSpec(inteventsv1.TopicSpec{
					Topic:             testTopicID,
					PropagationPolicy: "CreateDelete",
					EnablePublisher:   &falseVal,
				}),
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
				v1.WithPullSubscriptionSpec(inteventsv1.PullSubscriptionSpec{
					Topic: testTopicID,
					PubSubSpec: gcpduckv1.PubSubSpec{
						Secret: &secret,
						SourceSpec: duckv1.SourceSpec{
							Sink: newSinkDestination(),
						},
					},
					AdapterType: string(converters.CloudAuditLogs),
				})),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithCloudAuditLogsSourceSubscriptionID(v1.SubscriptionID),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSinkReady,
				v1.WithCloudAuditLogsSourceSinkID(testSinkID),
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		}},
	}, {
		Name: "sink delete fails",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSinkReady,
				v1.WithCloudAuditLogsSourceSinkID(testSinkID),
				v1.WithCloudAuditLogsSourceDeletionTimestamp,
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
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
			Eventf(corev1.EventTypeWarning, deleteSinkFailed, fmt.Sprintf("%s: delete-sink-induced-error", failedToDeleteSinkMsg)),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSinkUnknown(deleteSinkFailed, fmt.Sprintf("%s: delete-sink-induced-error", failedToDeleteSinkMsg)),
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSinkID(testSinkID),
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceDeletionTimestamp,
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		}},
	}, {
		Name: "sink delete succeeds",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSinkReady,
				v1.WithCloudAuditLogsSourceSinkID(testSinkID),
				v1.WithCloudAuditLogsSourceDeletionTimestamp,
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
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
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSinkDeleted,
				v1.WithCloudAuditLogsSourceTopicDeleted,
				v1.WithCloudAuditLogsSourcePullSubscriptionDeleted,
				v1.WithCloudAuditLogsSourceDeletionTimestamp,
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1", Resource: "topics"}},
				Name: sourceName,
			},
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1", Resource: "pullsubscriptions"}},
				Name: sourceName,
			},
		},
	}, {
		Name: "delete succeeds, sink does not exist",
		Objects: []runtime.Object{
			v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithCloudAuditLogsSourceProjectID(testProject),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceTopicReady(testTopicID),
				v1.WithCloudAuditLogsSourcePullSubscriptionReady,
				v1.WithCloudAuditLogsSourceSinkURI(calSinkURL),
				v1.WithCloudAuditLogsSourceSinkReady,
				v1.WithCloudAuditLogsSourceSinkID(testSinkID),
				v1.WithCloudAuditLogsSourceDeletionTimestamp,
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
			v1.NewTopic(sourceName, testNS,
				v1.WithTopicReady(testTopicID),
				v1.WithTopicAddress(testTopicURI),
				v1.WithTopicProjectID(testProject),
				v1.WithTopicSetDefaults,
			),
			v1.NewPullSubscription(sourceName, testNS,
				v1.WithPullSubscriptionReady(sinkURI),
			),
		},
		Key: testNS + "/" + sourceName,
		OtherTestData: map[string]interface{}{
			"expectedSinks": map[string]*logadmin.Sink{
				testSinkID: nil,
			},
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: v1.NewCloudAuditLogsSource(sourceName, testNS,
				v1.WithCloudAuditLogsSourceUID(sourceUID),
				v1.WithCloudAuditLogsSourceMethodName(testMethodName),
				v1.WithCloudAuditLogsSourceServiceName(testServiceName),
				v1.WithCloudAuditLogsSourceSink(sinkGVK, sinkName),
				v1.WithInitCloudAuditLogsSourceConditions,
				v1.WithCloudAuditLogsSourceSinkDeleted,
				v1.WithCloudAuditLogsSourceTopicDeleted,
				v1.WithCloudAuditLogsSourcePullSubscriptionDeleted,
				v1.WithCloudAuditLogsSourceDeletionTimestamp,
				v1.WithCloudAuditLogsSourceSetDefaults,
			),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1", Resource: "topics"}},
				Name: sourceName,
			},
			{ActionImpl: clientgotesting.ActionImpl{
				Namespace: testNS, Verb: "delete", Resource: schema.GroupVersionResource{Group: "internal.events.cloud.google.com", Version: "v1", Resource: "pullsubscriptions"}},
				Name: sourceName,
			},
		},
	}}

	for _, tt := range table {
		t.Run(tt.Name, func(t *testing.T) {
			logadminClientProvider := glogadmintesting.TestClientCreator(tt.OtherTestData["logadmin"])
			if existingSinks := tt.OtherTestData["existingSinks"]; existingSinks != nil {
				createSinks(t, logadminClientProvider, existingSinks.([]logadmin.Sink))
			}
			tt.Test(t, MakeFactory(
				func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
					r := &Reconciler{
						PubSubBase: intevents.NewPubSubBase(ctx,
							&intevents.PubSubBaseArgs{
								ControllerAgentName: controllerAgentName,
								ReceiveAdapterName:  receiveAdapterName,
								ReceiveAdapterType:  string(converters.CloudAuditLogs),
								ConfigWatcher:       cmw,
							}),
						Identity:               identity.NewIdentity(ctx, NoopIAMPolicyManager, NewGCPAuthTestStore(t, nil)),
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
