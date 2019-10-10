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
	"fmt"
	"strings"
	"testing"

	"github.com/google/knative-gcp/pkg/operations"
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"

	"knative.dev/pkg/kmeta"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	trueVal = true
)

const (
	notificationId = "124"
	projectId      = "test-project-here"
	storageUID     = "storageUID"
	bucket         = "testbucket"
	storageName    = "storageName"
	testNS         = "testns"
	testImage      = "testImage"
	secretName     = "google-cloud-key"
	credsMountPath = "/var/secrets/google"
	credsVolume    = "google-cloud-key"
	credsFile      = "/var/secrets/google/key.json"
	topicID        = "topicId"
)

var (
	backoffLimit = int32(3)
	parallelism  = int32(1)
)

// Returns an ownerref for the test Scheduler object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
		Kind:               "Storage",
		Name:               "my-test-storage",
		UID:                storageUID,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}

func TestValidateCreateArgs(t *testing.T) {
	tests := []struct {
		name        string
		f           func(NotificationCreateArgs) NotificationCreateArgs
		expectedErr string
	}{{
		name: "missing Bucket on create",
		f: func(a NotificationCreateArgs) NotificationCreateArgs {
			a.Bucket = ""
			return a
		},
		expectedErr: "missing Bucket",
	}, {
		name: "missing topicId on create",
		f: func(a NotificationCreateArgs) NotificationCreateArgs {
			a.TopicID = ""
			return a
		},
		expectedErr: "missing TopicID",
	}, {
		name: "missing ProjectID on create",
		f: func(a NotificationCreateArgs) NotificationCreateArgs {
			a.ProjectID = ""
			return a
		},
		expectedErr: "missing ProjectID",
	}, {
		name: "missing eventTypes on create",
		f: func(a NotificationCreateArgs) NotificationCreateArgs {
			a.EventTypes = nil
			return a
		},
		expectedErr: "missing EventTypes",
	}, {
		name:        "valid create",
		f:           func(a NotificationCreateArgs) NotificationCreateArgs { return a },
		expectedErr: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.f(validCreateArgs()).Validate()

			if (test.expectedErr != "" && got == nil) ||
				(test.expectedErr == "" && got != nil) ||
				(test.expectedErr != "" && got != nil && test.expectedErr != got.Error()) {
				t.Errorf("Error mismatch, want: %q got: %q", test.expectedErr, got)
			}
		})
	}
}

func TestValidateDeleteArgs(t *testing.T) {
	tests := []struct {
		name        string
		f           func(NotificationDeleteArgs) NotificationDeleteArgs
		expectedErr string
	}{{
		name:        "valid delete",
		f:           func(a NotificationDeleteArgs) NotificationDeleteArgs { return a },
		expectedErr: "",
	}, {
		name: "missing NotificationId on delete",
		f: func(a NotificationDeleteArgs) NotificationDeleteArgs {
			a.NotificationId = ""
			return a
		},
		expectedErr: "missing NotificationId",
	}, {
		name: "missing Bucket on delete",
		f: func(a NotificationDeleteArgs) NotificationDeleteArgs {
			a.Bucket = ""
			return a
		},
		expectedErr: "missing Bucket",
	}, {
		name: "missing ProjectID on delete",
		f: func(a NotificationDeleteArgs) NotificationDeleteArgs {
			a.ProjectID = ""
			return a
		},
		expectedErr: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.f(validDeleteArgs()).Validate()

			if (test.expectedErr != "" && got == nil) ||
				(test.expectedErr == "" && got != nil) ||
				(test.expectedErr != "" && got != nil && test.expectedErr != got.Error()) {
				t.Errorf("Error mismatch, want: %q got: %q", test.expectedErr, got)
			}
		})
	}
}

func TestNewNotificationOps(t *testing.T) {
	tests := []struct {
		name        string
		args        operations.JobArgs
		expectedErr string
		expected    *batchv1.Job
	}{{
		name:        "valid create",
		args:        validCreateArgs(),
		expectedErr: "",
		expected:    validCreateJob(),
	}, {
		name:        "valid delete",
		args:        validDeleteArgs(),
		expectedErr: "",
		expected:    validDeleteJob(),
	}}

	opCtx := operations.OpCtx{
		UID:   storageUID,
		Image: testImage,
		Secret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "key.json",
		},
		Owner: reconcilertesting.NewStorage(storageName, testNS),
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := operations.NewOpsJob(opCtx, test.args)

			if (test.expectedErr != "" && err == nil) ||
				(test.expectedErr == "" && err != nil) ||
				(test.expectedErr != "" && err != nil && test.expectedErr != err.Error()) {
				t.Errorf("Error mismatch, want: %q got: %q", test.expectedErr, err)
			}
			sortEnvVars := cmpopts.SortSlices(func(l corev1.EnvVar, r corev1.EnvVar) bool {
				return l.Name < r.Name
			})
			if diff := cmp.Diff(test.expected, got, sortEnvVars); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}

		})
	}
}

func validCreateArgs() NotificationCreateArgs {
	return NotificationCreateArgs{
		NotificationArgs: NotificationArgs{
			StorageArgs: StorageArgs{
				ProjectID: projectId,
			},
			Bucket: bucket,
		},
		EventTypes: []string{"finalize", "delete"},
		TopicID:    topicID,
	}
}

func validCreateJob() *batchv1.Job {
	owner := reconcilertesting.NewStorage(storageName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("storage-n-%s-storage-create", strings.ToLower(storageName)),
			Namespace: testNS,
			Labels: map[string]string{
				"events.cloud.google.com/notification": fmt.Sprintf("%s-Storage-createops", storageName),
			},
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     podTemplate("create"),
		},
	}
}

func validDeleteArgs() NotificationDeleteArgs {
	return NotificationDeleteArgs{
		NotificationArgs: NotificationArgs{
			StorageArgs: StorageArgs{},
			Bucket:      bucket,
		},
		NotificationId: notificationId,
	}
}

func validDeleteJob() *batchv1.Job {
	owner := reconcilertesting.NewStorage(storageName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("storage-n-%s-storage-delete", strings.ToLower(storageName)),
			Namespace: testNS,

			Labels: map[string]string{
				"events.cloud.google.com/notification": fmt.Sprintf("%s-Storage-deleteops", storageName),
			},
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     podTemplate("delete"),
		},
	}
}

func podTemplate(action string) corev1.PodTemplateSpec {
	env := []corev1.EnvVar{
		{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: credsFile,
		}, {
			Name:  "ACTION",
			Value: action,
		}, {
			Name:  "BUCKET",
			Value: bucket,
		},
	}
	switch action {
	case "create":
		createEnv := []corev1.EnvVar{
			{
				Name:  "EVENT_TYPES",
				Value: "finalize:delete",
			}, {
				Name:  "PUBSUB_TOPIC_ID",
				Value: topicID,
			}, {
				Name:  "PROJECT_ID",
				Value: projectId,
			},
		}
		env = append(env, createEnv...)
	case "delete":
		createEnv := []corev1.EnvVar{
			{
				Name:  "NOTIFICATION_ID",
				Value: notificationId,
			},
		}
		env = append(env, createEnv...)
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
			Labels: map[string]string{
				"resource-uid": storageUID,
				"action":       action,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "job",
				Image:           testImage,
				ImagePullPolicy: "Always",
				Env:             env,
				VolumeMounts: []corev1.VolumeMount{{
					Name:      secretName,
					MountPath: credsMountPath,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: credsVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			}},
		},
	}
}
