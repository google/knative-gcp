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
	"testing"

	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	//	. "knative.dev/pkg/reconciler/testing"
)

var (
	trueVal = true
)

const (
	validParent   = "projects/foo/locations/us-central1"
	validJobName  = validParent + "/jobs/myjobname"
	schedulerUID  = "schedulerUID"
	schedulerName = "schedulerName"
	testNS        = "testns"
)

// Returns an ownerref for the test Scheduler object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.run/v1alpha1",
		Kind:               "Scheduler",
		Name:               "my-test-scheduler",
		UID:                schedulerUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}

func TestValidateArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        JobArgs
		expectedErr string
	}{{
		name:        "empty",
		args:        JobArgs{},
		expectedErr: "missing UID",
	}, {
		name:        "missing Image",
		args:        JobArgs{UID: "uid"},
		expectedErr: "missing Image",
	}, {
		name: "missing JobName",
		args: JobArgs{
			UID:    "uid",
			Image:  "image",
			Action: "create",
		},
		expectedErr: "missing JobName",
	}, {
		name: "invalid JobName format",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "create",
			JobName: "projects/foo",
		},
		expectedErr: "JobName format is wrong",
	}, {
		name: "missing secret",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "create",
			JobName: validJobName,
		},
		expectedErr: "invalid secret missing name or key",
	}, {
		name: "missing owner",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
		},
		expectedErr: "missing owner",
	}, {
		name: "missing topicId",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewScheduler(schedulerName, testNS),
		},
		expectedErr: "missing TopicID",
	}, {
		name: "missing Schedule",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			Owner:   reconcilertesting.NewScheduler(schedulerName, testNS),
			TopicID: "topic",
		},
		expectedErr: "missing Schedule",
	}, {
		name: "valid",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			Owner:    reconcilertesting.NewScheduler(schedulerName, testNS),
			TopicID:  "topic",
			Schedule: "foo",
		},
		expectedErr: "",
	}, {
		name: "valid delete",
		args: JobArgs{
			UID:     "uid",
			Image:   "image",
			Action:  "delete",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewScheduler(schedulerName, testNS),
		},
		expectedErr: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := validateArgs(test.args)

			if (test.expectedErr != "" && got == nil) ||
				(test.expectedErr == "" && got != nil) ||
				(test.expectedErr != "" && got != nil && test.expectedErr != got.Error()) {
				t.Errorf("Error mismatch, want: %q got: %q", test.expectedErr, got)
			}
		})
	}
}
