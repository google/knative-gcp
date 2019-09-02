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

package operations

import (
	"context"
	"encoding/json"

	//	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	storageUID     = "test-storage-uid"
	bucket         = "my-test-bucket"
	notificationId = "135"

	testNS      = "testnamespace"
	testProject = "test-project-id"
)

type NotificationActionResult struct {
	// Result is the result the operation attempted.
	Result bool `json:"result,omitempty"`
	// Error is the error string if failure occurred
	Error string `json:"error,omitempty"`
	// NotificationId holds the notification ID for GCS
	// and is filled in during create operation.
	NotificationId string `json:"notificationId,omitempty"`
	// Project is the project id that we used (this might have
	// been defaulted, to we'll expose it).
	ProjectId string `json:"projectId,omitempty"`
}

func TestGetNotificationActionResult(t *testing.T) {
	narSuccess := NotificationActionResult{
		Result:         true,
		NotificationId: notificationId,
		ProjectId:      testProject,
	}
	narFailure := NotificationActionResult{
		Result:    false,
		Error:     "test induced failure",
		ProjectId: testProject,
	}

	successMsg, err := json.Marshal(narSuccess)
	if err != nil {
		t.Fatalf("Failed to marshal success NotificationActionResult: %s", err)
	}

	failureMsg, err := json.Marshal(narFailure)
	if err != nil {
		t.Fatalf("Failed to marshal failure NotificationActionResult: %s", err)
	}

	testCases := []struct {
		name           string
		pod            *corev1.Pod
		expectedResult NotificationActionResult
		expectedErr    string
	}{{
		name:           "nil pod",
		pod:            nil,
		expectedResult: NotificationActionResult{},
		expectedErr:    "pod was nil",
	}, {
		name:           "no termination message",
		pod:            newPod(""),
		expectedResult: NotificationActionResult{},
		expectedErr:    `did not find termination message for pod "test-pod"`,
	}, {
		name:           "garbage termination message",
		pod:            newPod("garbage"),
		expectedResult: NotificationActionResult{},
		expectedErr:    `failed to unmarshal terminationmessage: "garbage" : "invalid character 'g' looking for beginning of value"`,
	}, {
		name:           "action failed",
		pod:            newPod(string(failureMsg)),
		expectedResult: narFailure,
		expectedErr:    "",
	}, {
		name:           "action succeeded",
		pod:            newPod(string(successMsg)),
		expectedResult: narSuccess,
		expectedErr:    "",
	}}

	for _, tc := range testCases {
		var nc NotificationActionResult
		err := GetOperationsResult(context.TODO(), tc.pod, &nc)
		if (tc.expectedErr != "" && err == nil) ||
			(tc.expectedErr == "" && err != nil) ||
			(tc.expectedErr != "" && err != nil && tc.expectedErr != err.Error()) {
			t.Errorf("Error mismatch, want: %q got: %q", tc.expectedErr, err)
		}
		if diff := cmp.Diff(tc.expectedResult, nc); diff != "" {
			t.Errorf("unexpected action result (-want, +got) = %v", diff)
		}
	}
}

func newPod(msg string) *corev1.Pod {
	labels := map[string]string{
		"resource-uid": storageUID,
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
