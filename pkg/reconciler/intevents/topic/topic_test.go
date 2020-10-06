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

package topic

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"

	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	pubsubv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1/topic"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/topic/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	topicName = "hubbub"
	sinkName  = "sink"

	testNS       = "testnamespace"
	testImage    = "test_image"
	topicUID     = topicName + "-abc-123"
	testProject  = "test-project-id"
	testTopicID  = "cloud-run-topic-" + testNS + "-" + topicName + "-" + topicUID
	testTopicURI = "http://" + topicName + "-topic." + testNS + ".svc.cluster.local"

	secretName = "testing-secret"

	failedToReconcileTopicMsg = `Failed to reconcile Pub/Sub topic`
	failedToDeleteTopicMsg    = `Failed to delete Pub/Sub topic`
)

var (
	trueVal  = true
	falseVal = false

	sinkDNS = sinkName + ".mynamespace.svc.cluster.local"
	sinkURI = "http://" + sinkDNS + "/"

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1",
		Kind:    "Sink",
	}

	secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secretName,
		},
		Key: "testing-key",
	}
)

func init() {
	// Add types to scheme
	_ = pubsubv1.AddToScheme(scheme.Scheme)
}

func newSink() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "testing.cloud.google.com/v1",
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

func newSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      secretName,
		},
		Data: map[string][]byte{
			"testing-key": []byte("abcd"),
		},
	}
}

func TestAllCases(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "create client fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("NoCreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		OtherTestData: map[string]interface{}{
			"client-error": "create-client-induced-error",
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, reconciledTopicFailedReason, "Failed to reconcile Pub/Sub topic: create-client-induced-error"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileTopicMsg, "create-client-induced-error")),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
	}, {
		Name: "verify topic exists fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("NoCreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		OtherTestData: map[string]interface{}{
			// GetTopic has a retry policy for Unknown status type, so we use Internal error instead.
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("GetTopic", codes.Internal, "Injected error")},
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, reconciledTopicFailedReason, "Failed to reconcile Pub/Sub topic: rpc error: code = Internal desc = Injected error"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileTopicMsg, "rpc error: code = Internal desc = Injected error")),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
	}, {
		Name: "topic does not exist and propagation policy is NoCreateNoDelete",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("NoCreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, reconciledTopicFailedReason, "Failed to reconcile Pub/Sub topic: Topic %q does not exist and the topic policy doesn't allow creation", testTopicID),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("NoCreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: Topic %q does not exist and the topic policy doesn't allow creation", failedToReconcileTopicMsg, testTopicID)),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
		},
	}, {
		Name: "create topic fails",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeWarning, reconciledTopicFailedReason, "Failed to reconcile Pub/Sub topic: rpc error: code = Unknown desc = Injected error"),
		},
		OtherTestData: map[string]interface{}{
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("CreateTopic", codes.Unknown, "Injected error")},
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicNoTopic("TopicReconcileFailed", fmt.Sprintf("%s: %s", failedToReconcileTopicMsg, "rpc error: code = Unknown desc = Injected error")),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
		},
	}, {
		Name: "topic created with EnablePublisher = false",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project:         testProject,
					Topic:           testTopicID,
					Secret:          &secret,
					EnablePublisher: &falseVal,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project:         testProject,
					Topic:           testTopicID,
					Secret:          &secret,
					EnablePublisher: &falseVal,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReady(testTopicID),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "publisher has not yet been reconciled",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WantCreates: []runtime.Object{
			newPublisher(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicPublisherNotConfigured,
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "the status of publisher is false",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			ProvideResource("create", "services", makeFalseStatusPublisher("PublisherNotDeployed", "PublisherNotDeployed")),
		},
		WantCreates: []runtime.Object{
			newPublisher(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicPublisherNotDeployed("PublisherNotDeployed", "PublisherNotDeployed"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "the status of publisher is unknown",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			ProvideResource("create", "services", makeUnknownStatusPublisher("PublisherUnknown", "PublisherUnknown")),
		},
		WantCreates: []runtime.Object{
			newPublisher(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicPublisherUnknown("PublisherUnknown", "PublisherUnknown"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "topic successfully reconciles and is ready",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			ProvideResource("create", "services", makeReadyPublisher()),
		},
		WantCreates: []runtime.Object{
			newPublisher(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicPublisherDeployed,
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExistsWithConfig(testTopicID, &pubsub.TopicConfig{}),
		},
	}, {
		Name: "topic successfully reconciles and reuses existing publisher",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
			makeReadyPublisher(),
			NewService(topicName+"-topic", testNS,
				WithServiceOwnerReferences(ownerReferences()),
				WithServiceLabels(resources.GetLabels(controllerAgentName, topicName)),
				WithServicePorts(servicePorts())),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WithReactors: []clientgotesting.ReactionFunc{},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicPublisherDeployed,
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "delete topic - policy CreateNoDelete",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicTopicID(testTopicID),
				reconcilertestingv1.WithTopicDeleted,
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		Key:               testNS + "/" + topicName,
		WantEvents:        nil,
		WantStatusUpdates: nil,
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "delete topic - policy CreateDelete",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateDelete"),
				reconcilertestingv1.WithTopicTopicID(testTopicID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicDeleted,
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
		},
		Key:               testNS + "/" + topicName,
		WantEvents:        nil,
		WantStatusUpdates: nil,
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
		},
	}, {
		Name: "fail to delete - policy CreateDelete",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateDelete"),
				reconcilertestingv1.WithTopicTopicID(testTopicID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicDeleted,
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, deleteTopicFailed, "Failed to delete Pub/Sub topic: rpc error: code = Unknown desc = Injected error"),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				Topic(testTopicID),
			},
			"server-options": []pstest.ServerReactorOption{pstest.WithErrorInjection("DeleteTopic", codes.Unknown, "Injected error")},
		},
		WantStatusUpdates: nil,
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists(testTopicID),
		},
	}, {
		Name: "topic successfully reconciles with data residency config",
		Objects: []runtime.Object{
			reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				reconcilertestingv1.WithTopicSetDefaults,
			),
			newSink(),
			newSecret(),
		},
		Key: testNS + "/" + topicName,
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, topicName, resourceGroup),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "FinalizerUpdate", "Updated %q finalizers", topicName),
			Eventf(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, testNS, topicName),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			ProvideResource("create", "services", makeReadyPublisher()),
		},
		WantCreates: []runtime.Object{
			newPublisher(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilertestingv1.NewTopic(topicName, testNS,
				reconcilertestingv1.WithTopicUID(topicUID),
				reconcilertestingv1.WithTopicProjectID(testProject),
				reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
					Project: testProject,
					Topic:   testTopicID,
					Secret:  &secret,
				}),
				reconcilertestingv1.WithTopicPropagationPolicy("CreateNoDelete"),
				// Updates
				reconcilertestingv1.WithInitTopicConditions,
				reconcilertestingv1.WithTopicReadyAndPublisherDeployed(testTopicID),
				reconcilertestingv1.WithTopicPublisherDeployed,
				reconcilertestingv1.WithTopicAddress(testTopicURI),
				reconcilertestingv1.WithTopicSetDefaults,
			),
		}},
		OtherTestData: map[string]interface{}{
			"pre":                    []PubsubAction{},
			"dataResidencyConfigMap": NewDataresidencyConfigMapFromRegions([]string{"us-east1"}),
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExistsWithConfig(testTopicID, &pubsub.TopicConfig{
				MessageStoragePolicy: pubsub.MessageStoragePolicy{
					AllowedPersistenceRegions: []string{"us-east1"},
				},
			}),
		},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		// Insert pubsub client for PostConditions and create fixtures
		opts := []pstest.ServerReactorOption{}
		if testData != nil {
			if testData["server-options"] != nil {
				opts = testData["server-options"].([]pstest.ServerReactorOption)
			}
		}

		srv := pstest.NewServer(opts...)

		psclient, _ := GetTestClientCreateFunc(srv.Addr)(ctx, testProject)
		conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
		if err != nil {
			panic(fmt.Errorf("failed to dial test pubsub connection: %v", err))
		}
		close := func() {
			srv.Close()
			conn.Close()
		}
		t.Cleanup(close)
		if testData != nil {
			InjectPubsubClient(testData, psclient)
			if testData["pre"] != nil {
				fixtures := testData["pre"].([]PubsubAction)
				for _, f := range fixtures {
					f(ctx, t, psclient)
				}
			}
		}

		// If we found "dataResidencyConfigMap" in OtherData, we create a store with the configmap
		var drStore *dataresidency.Store
		if cm, ok := testData["dataResidencyConfigMap"]; ok {
			drStore = NewDataresidencyTestStore(t, cm.(*corev1.ConfigMap))
		}
		// use normal create function or always error one
		var createClientFn reconcilerutilspubsub.CreateFn
		if testData != nil && testData["client-error"] != nil {
			createClientFn = func(ctx context.Context, projectID string, opts ...option.ClientOption) (*pubsub.Client, error) {
				return nil, fmt.Errorf(testData["client-error"].(string))
			}
		} else {
			createClientFn = GetTestClientCreateFunc(srv.Addr)
		}
		pubsubBase := &intevents.PubSubBase{
			Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
		}
		r := &Reconciler{
			PubSubBase:         pubsubBase,
			topicLister:        listers.GetTopicLister(),
			serviceLister:      listers.GetV1ServiceLister(),
			publisherImage:     testImage,
			createClientFn:     createClientFn,
			dataresidencyStore: drStore,
		}
		return topic.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetTopicLister(), r.Recorder, r)
	}))

}

func ProvideResource(verb, resource string, obj runtime.Object) clientgotesting.ReactionFunc {
	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if !action.Matches(verb, resource) {
			return false, nil, nil
		}
		return true, obj, nil
	}
}

func patchFinalizers(namespace, name, finalizer string, existingFinalizers ...string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	for i, ef := range existingFinalizers {
		existingFinalizers[i] = fmt.Sprintf("%q", ef)
	}
	if finalizer != "" {
		existingFinalizers = append(existingFinalizers, fmt.Sprintf("%q", finalizer))
	}
	fname := strings.Join(existingFinalizers, ",")
	patch := `{"metadata":{"finalizers":[` + fname + `],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func ownerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         "internal.events.cloud.google.com/reconcilertestingv1",
		Kind:               "Topic",
		Name:               topicName,
		UID:                topicUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}}
}

func servicePorts() []corev1.ServicePort {
	svcPorts := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(8080),
		}, {
			Name: "metrics",
			Port: 9090,
		},
	}
	return svcPorts
}

func makeReadyPublisher() *servingv1.Service {
	pub := newPublisher()
	pub.Status.Conditions = []apis.Condition{{
		Type:   apis.ConditionReady,
		Status: "True",
	}}
	uri, _ := apis.ParseURL(testTopicURI)
	pub.Status.Address = &duckv1.Addressable{
		URL: uri,
	}
	return pub
}

func makeUnknownStatusPublisher(reason, message string) *servingv1.Service {
	pub := newPublisher()
	pub.Status.Conditions = []apis.Condition{{
		Type:    apis.ConditionReady,
		Status:  "Unknown",
		Reason:  reason,
		Message: message,
	}}
	return pub
}

func makeFalseStatusPublisher(reason, message string) *servingv1.Service {
	pub := newPublisher()
	pub.Status.Conditions = []apis.Condition{{
		Type:    apis.ConditionReady,
		Status:  "False",
		Reason:  reason,
		Message: message,
	}}
	return pub
}

func newPublisher() *servingv1.Service {
	t := reconcilertestingv1.NewTopic(topicName, testNS,
		reconcilertestingv1.WithTopicUID(topicUID),
		reconcilertestingv1.WithTopicSpec(pubsubv1.TopicSpec{
			Project: testProject,
			Topic:   testTopicID,
			Secret:  &secret,
		}),
		reconcilertestingv1.WithTopicSetDefaults,
	)
	args := &resources.PublisherArgs{
		Image:  testImage,
		Topic:  t,
		Labels: resources.GetLabels(controllerAgentName, topicName),
	}
	return resources.MakePublisher(args)
}
