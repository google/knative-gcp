/*
Copyright 2021 Google LLC

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

package handlers

import (
	"context"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"knative.dev/eventing/pkg/apis/sources"
	"knative.dev/pkg/logging"
)

const (
	// ApiServerSourceCreateProbeEventType is the CloudEvent type of forward
	// ApiServerSource create probes.
	ApiServerSourceCreateProbeEventType = "apiserversource-probe-create"

	// ApiServerSourceUpdateProbeEventType is the CloudEvent type of forward
	// ApiServerSource update probes.
	ApiServerSourceUpdateProbeEventType = "apiserversource-probe-update"

	// ApiServerSourceDeleteProbeEventType is the CloudEvent type of forward
	// ApiServerSource delete probes.
	ApiServerSourceDeleteProbeEventType = "apiserversource-probe-delete"

	testPodName = "apiserversource-test-pod"
)

func NewApiServerSourceProbe(projectID clients.ProjectID, k8sClient kubernetes.Interface) *ApiServerSourceProbe {
	return &ApiServerSourceProbe{
		projectID:      projectID,
		k8sClient:      k8sClient,
		receivedEvents: utils.NewSyncReceivedEvents(),
	}
}

// ApiServerSourceProbe is the probe handler for probe requests in the
// ApiServerSource probe.
type ApiServerSourceProbe struct {
	// The project ID
	projectID clients.ProjectID

	// Kubernetes client used to create a test pod
	k8sClient kubernetes.Interface

	// The map of received events to be tracked by the forwarder and receiver
	receivedEvents *utils.SyncReceivedEvents
}

type ApiServerSourceCreateProbe struct {
	*ApiServerSourceProbe
}

type ApiServerSourceUpdateProbe struct {
	*ApiServerSourceProbe
}

type ApiServerSourceDeleteProbe struct {
	*ApiServerSourceProbe
}

// Forward creates a pod in order to generate an ApiServerSource notification event.
func (p *ApiServerSourceCreateProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	namespace := fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension])[1:]
	channelID := channelID(namespace, event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe creates a pod.
	podName := fmt.Sprintf("%s.%s", testPodName, event.ID()[len(event.Type())+1:])
	logging.FromContext(ctx).Infow("Creating pod", zap.String("podName", podName))
	_, err = p.k8sClient.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to create test pod: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Forward updates a pod in order to generate an ApiServerSource notification event.
func (p *ApiServerSourceUpdateProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	namespace := fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension])[1:]
	channelID := channelID(namespace, event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe updates a pod.
	podName := fmt.Sprintf("%s.%s", testPodName, event.ID()[len(event.Type())+1:])
	logging.FromContext(ctx).Infow("Updating pod", zap.String("podName", podName))

	_, err = p.k8sClient.CoreV1().Pods(namespace).Patch(ctx, podName, types.JSONPatchType, []byte(`[{"op": "replace", "path": "/spec/containers/0/image", "value":"alpine"}]`), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update test pod: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Forward deletes a pod in order to generate an ApiServerSource notification event.
func (p *ApiServerSourceDeleteProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	namespace := fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension])[1:]
	channelID := channelID(namespace, event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe deletes a pod.
	podName := fmt.Sprintf("%s.%s", testPodName, event.ID()[len(event.Type())+1:])
	logging.FromContext(ctx).Infow("Deleting pod", zap.String("podName", podName))
	err = p.k8sClient.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete test pod: %v", err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Receive closes the receiver channel associated with an ApiServerSource notification event.
func (p *ApiServerSourceProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: dev.knative.apiserver.resource.add
	//     source: https://10.96.0.1:443
	//     subject: /apis/v1/namespaces/default/events/testevents.15dd3050eb1e6f50
	//     id: e0447eb7-36b5-443b-9d37-faf4fe5c62f0
	//     time: 2020-07-28T16:35:14.172979816Z
	//     datacontenttype: application/json
	//   Extensions,
	//     kind: Pod
	//     name: busybox
	//     namespace: default
	//   Data,
	//     { ... }
	var forwardType string
	switch event.Type() {
	case sources.ApiServerSourceAddEventType:
		forwardType = ApiServerSourceCreateProbeEventType
	case sources.ApiServerSourceUpdateEventType:
		forwardType = ApiServerSourceUpdateProbeEventType
	case sources.ApiServerSourceDeleteEventType:
		forwardType = ApiServerSourceDeleteProbeEventType
	default:
		return fmt.Errorf("Unrecognized ApiServerSource event type: %s", event.Type())
	}
	nameExtension := fmt.Sprint(event.Extensions()["name"])
	sepNameExtension := strings.Split(nameExtension, ".")
	if len(sepNameExtension) != 2 {
		return fmt.Errorf("Failed to read ApiServer event, unexpected name extension: %s", nameExtension)
	}
	eventID := sepNameExtension[1]
	eventID = fmt.Sprintf("%s-%s", forwardType, eventID)
	namespace := fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension])[1:]
	channelID := channelID(namespace, eventID)
	if err := p.receivedEvents.SignalReceiverChannel(channelID); err != nil {
		return err
	}
	logging.FromContext(ctx).Info("Successfully received ApiServerSource probe event")
	return nil
}
