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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	testiam "github.com/google/knative-gcp/pkg/gclient/iam/testing"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
	glogadmintesting "github.com/google/knative-gcp/pkg/gclient/logging/logadmin/testing"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub/testing"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
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
)

const (
	sourceName        = "test-auditlogssource"
	sourceUID         = "test-auditlogssource-uid"
	testNS            = "testnamespace"
	testProject       = "test-project-id"
	testTopicID       = "auditlogssource-" + sourceUID
	testTopicResource = "pubsub.googleapis.com/projects/" + testProject + "/topics/" + testTopicID
	testTopicURI      = "http://" + sourceName + "-topic." + testNS + ".svc.cluster.local"
	testSinkID        = "sink-" + sourceUID

	testServiceName = "test-service"
	testMethodName  = "test-method"
	testFilter      = `protoPayload.methodName="test-method" AND protoPayload.serviceName="test-service" AND protoPayload."@type"="type.googleapis.com/google.cloud.audit.AuditLog"`

	sinkName = "sink"
	sinkDNS  = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI  = "http://" + sinkDNS + "/"

	topicNotReadyMsg            = `Topic "test-auditlogssource" not ready`
	pullSubscriptionNotReadyMsg = `PullSubscription "test-auditlogssource" not ready`
	failedToCreateSinkMsg       = `failed to ensure creation of logging sink`
	failedToSetPermissionsMsg   = `failed to ensure sink has pubsub.publisher permission on source topic`
	failedToDeleteSinkMsg       = `Failed to delete Stackdriver sink`
)

var (
	trueVal = true

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

func sourceOwnerRef(name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "AuditLogsSource",
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
		fname = fmt.Sprintf("%q", finalizerName)
	}
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
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
	calSinkURL := sinkURL(t, sinkURI)

	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "topic created, not ready",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicNotReady("TopicNotReady", topicNotReadyMsg)),
		}},
		WantCreates: []runtime.Object{
			NewTopic(sourceName, testNS,
				WithTopicSpec(pubsubv1alpha1.TopicSpec{
					Topic:             "auditlogssource-" + sourceUID,
					PropagationPolicy: "CreateDelete",
				}),
				WithTopicLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				WithTopicOwnerReferences([]metav1.OwnerReference{sourceOwnerRef(sourceName, sourceUID)}),
			),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, true),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q finalizers", sourceName),
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q not ready", sourceName),
		},
	}, {
		Name: "topic exists, topic not ready",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
			),
			NewTopic(sourceName, testNS,
				WithTopicTopicID(testTopicID),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicNotReady("TopicNotReady", topicNotReadyMsg)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q not ready", sourceName),
		},
	}, {
		Name: "topic exists and is ready, no projectid",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicNotReady("TopicNotReady", `Topic "test-auditlogssource" did not expose projectid`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose projectid", sourceName),
		},
	}, {
		Name: "topic exists and is ready, no topicid",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(""),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicNotReady("TopicNotReady", `Topic "test-auditlogssource" did not expose topicid`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "Topic %q did not expose topicid", sourceName),
		},
	}, {
		Name: "topic exists and is ready, unexpected topicid",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady("garbaaaaage"),
				WithTopicProjectID(testProject),
				WithTopicAddress(testTopicURI),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicNotReady("TopicNotReady", `Topic "test-auditlogssource" mismatch: expected "auditlogssource-test-auditlogssource-uid" got "garbaaaaage"`),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `Topic %q mismatch: expected "auditlogssource-test-auditlogssource-uid" got "garbaaaaage"`, sourceName),
		},
	}, {
		Name: "topic exists and is ready, pullsubscription created",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
		WantCreates: []runtime.Object{
			NewPullSubscriptionWithNoDefaults(sourceName, testNS,
				WithPullSubscriptionSpecWithNoDefaults(pubsubv1alpha1.PullSubscriptionSpec{
					Topic:       testTopicID,
					Secret:      &secret,
					AdapterType: converters.AuditLogConverter,
				}),
				WithPullSubscriptionSink(sinkGVK, sinkName),
				WithPullSubscriptionLabels(map[string]string{
					"receive-adapter": receiveAdapterName,
				}),
				WithPullSubscriptionAnnotations(map[string]string{
					"metrics-resource-group": resourceGroup,
				}),
				WithPullSubscriptionOwnerReferences([]metav1.OwnerReference{sourceOwnerRef(sourceName, sourceUID)}),
			),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q not ready", sourceName),
		},
	}, {
		Name: "topic exists and ready, pullsubscription exists but is not ready",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
			),
			NewTopic(sourceName, testNS,
				WithTopicReady(testTopicID),
				WithTopicAddress(testTopicURI),
				WithTopicProjectID(testProject),
			),
			NewPullSubscriptionWithNoDefaults(sourceName, testNS),
		},
		Key:     testNS + "/" + sourceName,
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionNotReady("PullSubscriptionNotReady", pullSubscriptionNotReadyMsg),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "PullSubscription %q not ready", sourceName),
		},
	}, {
		Name: "logging client create fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "get sink fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "create sink fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkCreateFailed", "%s: %s", failedToCreateSinkMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, pubsub client create fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, get pubsub IAM policy fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceServiceName(testServiceName),
				WithAuditLogsSourceMethodName(testMethodName)),
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceServiceName(testServiceName),
				WithAuditLogsSourceMethodName(testMethodName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created, set pubsub IAM policy fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceServiceName(testServiceName),
				WithAuditLogsSourceMethodName(testMethodName)),
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "create-client-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceServiceName(testServiceName),
				WithAuditLogsSourceMethodName(testMethodName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkNotPublisher", "%s: %s", failedToSetPermissionsMsg, "create-client-induced-error"),
			),
		}},
	}, {
		Name: "sink created",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceServiceName(testServiceName),
				WithAuditLogsSourceMethodName(testMethodName)),
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
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ReadinessChanged", "AuditLogsSource %q became ready", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceServiceName(testServiceName),
				WithAuditLogsSourceMethodName(testMethodName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkReady(),
				WithAuditLogsSourceSinkID(testSinkID),
			),
		}},
	}, {
		Name: "sink exists",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
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
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "ReadinessChanged", "AuditLogsSource %q became ready", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkReady(),
				WithAuditLogsSourceSinkID(testSinkID),
			),
		}},
	}, {
		Name: "sink delete fails",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkReady(),
				WithAuditLogsSourceSinkID(testSinkID),
				WithAuditLogsSourceDeletionTimestamp,
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
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "delete-sink-induced-error"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkNotReady("SinkDeleteFailed", "%s: %s", failedToDeleteSinkMsg, "delete-sink-induced-error"),
				WithAuditLogsSourceSinkID(testSinkID),
				WithAuditLogsSourceDeletionTimestamp,
			),
		}},
	}, {
		Name: "sink delete succeeds",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkReady(),
				WithAuditLogsSourceSinkID(testSinkID),
				WithAuditLogsSourceDeletionTimestamp,
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
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceSinkNotReady("SinkDeleted", "Successfully deleted Stackdriver sink: %s", testSinkID),
				WithAuditLogsSourceTopicNotReady("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", sourceName)),
				WithAuditLogsSourcePullSubscriptionNotReady("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", sourceName)),
				WithAuditLogsSourceDeletionTimestamp,
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
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, false),
		},
	}, {
		Name: "delete succeeds, sink does not exist",
		Objects: []runtime.Object{
			NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithAuditLogsSourceProjectID(testProject),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceTopicReady(testTopicID),
				WithAuditLogsSourcePullSubscriptionReady(),
				WithAuditLogsSourceSinkURI(calSinkURL),
				WithAuditLogsSourceSinkReady(),
				WithAuditLogsSourceSinkID(testSinkID),
				WithAuditLogsSourceDeletionTimestamp,
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
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q finalizers", sourceName),
			Eventf(corev1.EventTypeNormal, "Updated", "Updated AuditLogsSource %q", sourceName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewAuditLogsSource(sourceName, testNS,
				WithAuditLogsSourceFinalizers(finalizerName),
				WithAuditLogsSourceSink(sinkGVK, sinkName),
				WithInitAuditLogsSourceConditions,
				WithAuditLogsSourceSinkNotReady("SinkDeleted", "Successfully deleted Stackdriver sink: %s", testSinkID),
				WithAuditLogsSourceTopicNotReady("TopicDeleted", fmt.Sprintf("Successfully deleted Topic: %s", sourceName)),
				WithAuditLogsSourcePullSubscriptionNotReady("PullSubscriptionDeleted", fmt.Sprintf("Successfully deleted PullSubscription: %s", sourceName)),
				WithAuditLogsSourceDeletionTimestamp,
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
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, sourceName, false),
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
					return &Reconciler{
						PubSubBase:             pubsub.NewPubSubBaseWithAdapter(ctx, controllerAgentName, receiveAdapterName, converters.AuditLogConverter, cmw),
						auditLogsSourceLister:  listers.GetAuditLogsSourceLister(),
						logadminClientProvider: logadminClientProvider,
						pubsubClientProvider:   gpubsub.TestClientCreator(testData["pubsub"]),
					}
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
