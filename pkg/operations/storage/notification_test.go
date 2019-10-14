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

	"knative.dev/pkg/kmeta"

	"github.com/google/go-cmp/cmp"

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

func TestValidateArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        NotificationArgs
		expectedErr string
	}{{
		name:        "empty",
		args:        NotificationArgs{},
		expectedErr: "missing UID",
	}, {
		name:        "missing Image",
		args:        NotificationArgs{UID: "uid"},
		expectedErr: "missing Image",
	}, {
		name: "missing Action",
		args: NotificationArgs{
			UID:   "uid",
			Image: testImage,
		},
		expectedErr: "missing Action",
	}, {
		name: "missing Bucket",
		args: NotificationArgs{
			UID:       "uid",
			Image:     testImage,
			Action:    "create",
			ProjectID: projectId,
		},
		expectedErr: "missing Bucket",
	}, {
		name: "missing secret",
		args: NotificationArgs{
			UID:       "uid",
			Image:     testImage,
			Action:    "create",
			ProjectID: projectId,
			Bucket:    bucket,
		},
		expectedErr: "invalid secret missing name or key",
	}, {
		name: "missing owner",
		args: NotificationArgs{
			UID:       "uid",
			Image:     testImage,
			Action:    "create",
			ProjectID: projectId,
			Bucket:    bucket,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
		},
		expectedErr: "missing owner",
	}, {
		name: "missing topicId on create",
		args: NotificationArgs{
			UID:       "uid",
			Image:     testImage,
			Action:    "create",
			ProjectID: projectId,
			Bucket:    bucket,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewStorage(storageName, testNS),
		},
		expectedErr: "missing TopicID",
	}, {
		name: "missing ProjectID on create",
		args: NotificationArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			Bucket:  bucket,
			TopicID: topicID,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewStorage(storageName, testNS),
		},
		expectedErr: "missing ProjectID",
	}, {
		name: "missing eventTypes on create",
		args: NotificationArgs{
			UID:       "uid",
			Image:     testImage,
			Action:    "create",
			ProjectID: projectId,
			Bucket:    bucket,
			TopicID:   topicID,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewStorage(storageName, testNS),
		},
		expectedErr: "missing EventTypes",
	}, {
		name: "missing NotificationId on delete",
		args: NotificationArgs{
			UID:       "uid",
			Image:     testImage,
			Action:    "delete",
			ProjectID: projectId,
			Bucket:    bucket,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewStorage(storageName, testNS),
		},
		expectedErr: "missing NotificationId",
	}, {
		name: "valid create",
		args: NotificationArgs{
			UID:        "uid",
			Image:      testImage,
			Action:     "create",
			ProjectID:  projectId,
			Bucket:     bucket,
			EventTypes: []string{"finalize", "delete"},
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner:   reconcilertesting.NewStorage(storageName, testNS),
			TopicID: topicID,
		},
		expectedErr: "",
	}, {
		name: "valid delete",
		args: NotificationArgs{
			UID:    "uid",
			Image:  testImage,
			Action: "delete",
			Bucket: bucket,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner:          reconcilertesting.NewStorage(storageName, testNS),
			NotificationId: notificationId,
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

func TestNewNotificationOps(t *testing.T) {
	tests := []struct {
		name        string
		args        NotificationArgs
		expectedErr string
		expected    *batchv1.Job
	}{{
		name:        "empty",
		args:        NotificationArgs{},
		expectedErr: "missing UID",
		expected:    nil,
	}, {
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewNotificationOps(test.args)

			if (test.expectedErr != "" && err == nil) ||
				(test.expectedErr == "" && err != nil) ||
				(test.expectedErr != "" && err != nil && test.expectedErr != err.Error()) {
				t.Errorf("Error mismatch, want: %q got: %q", test.expectedErr, err)
			}
			if diff := cmp.Diff(test.expected, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}

		})
	}
}

func validCreateArgs() NotificationArgs {
	return NotificationArgs{
		UID:        storageUID,
		Image:      testImage,
		Action:     "create",
		ProjectID:  projectId,
		Bucket:     bucket,
		EventTypes: []string{"finalize"},
		Secret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "key.json",
		},
		Owner:   reconcilertesting.NewStorage(storageName, testNS),
		TopicID: topicID,
	}
}

func validCreateJob() *batchv1.Job {
	owner := reconcilertesting.NewStorage(storageName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            NotificationJobName(owner, "create"),
			Namespace:       testNS,
			Labels:          NotificationJobLabels(owner, "create"),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     podTemplate("create"),
		},
	}
}

func validDeleteArgs() NotificationArgs {
	return NotificationArgs{
		UID:            storageUID,
		Image:          testImage,
		Action:         "delete",
		NotificationId: notificationId,
		Bucket:         bucket,
		Secret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "key.json",
		},
		Owner: reconcilertesting.NewStorage(storageName, testNS),
	}
}

func validDeleteJob() *batchv1.Job {
	owner := reconcilertesting.NewStorage(storageName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            NotificationJobName(owner, "delete"),
			Namespace:       testNS,
			Labels:          NotificationJobLabels(owner, "delete"),
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
				Value: "finalize",
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
