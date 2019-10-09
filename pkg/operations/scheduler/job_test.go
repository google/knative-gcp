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

	"github.com/google/go-cmp/cmp"
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"

	"knative.dev/pkg/kmeta"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	//	. "knative.dev/pkg/reconciler/testing"
)

var (
	trueVal = true
)

const (
	validParent         = "projects/foo/locations/us-central1"
	validJobName        = validParent + "/jobs/myjobname"
	schedulerUID        = "schedulerUID"
	schedulerName       = "schedulerName"
	testNS              = "testns"
	testImage           = "testImage"
	secretName          = "google-cloud-key"
	credsMountPath      = "/var/secrets/google"
	credsVolume         = "google-cloud-key"
	credsFile           = "/var/secrets/google/key.json"
	topicID             = "topicId"
	onceAMinuteSchedule = "* * * * *"
	testData            = "mytest data goes here"
)

var (
	backoffLimit = int32(3)
	parallelism  = int32(1)
)

// Returns an ownerref for the test Scheduler object
func ownerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "events.cloud.google.com/v1alpha1",
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
			Image:  testImage,
			Action: "create",
		},
		expectedErr: "missing JobName",
	}, {
		name: "invalid JobName format",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: "projects/foo",
		},
		expectedErr: "JobName format is wrong",
	}, {
		name: "missing secret",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: validJobName,
		},
		expectedErr: "invalid secret missing name or key",
	}, {
		name: "missing owner",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
		},
		expectedErr: "missing owner",
	}, {
		name: "missing topicId",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner: reconcilertesting.NewScheduler(schedulerName, testNS),
		},
		expectedErr: "missing TopicID",
	}, {
		name: "missing Schedule",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner:   reconcilertesting.NewScheduler(schedulerName, testNS),
			TopicID: topicID,
		},
		expectedErr: "missing Schedule",
	}, {
		name: "missing data",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner:    reconcilertesting.NewScheduler(schedulerName, testNS),
			TopicID:  topicID,
			Schedule: onceAMinuteSchedule,
		},
		expectedErr: "missing Data",
	}, {
		name: "valid create",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "create",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "key.json",
			},
			Owner:    reconcilertesting.NewScheduler(schedulerName, testNS),
			TopicID:  topicID,
			Schedule: onceAMinuteSchedule,
			Data:     testData,
		},
		expectedErr: "",
	}, {
		name: "valid delete",
		args: JobArgs{
			UID:     "uid",
			Image:   testImage,
			Action:  "delete",
			JobName: validJobName,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
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

func TestNewJobOps(t *testing.T) {
	tests := []struct {
		name        string
		args        JobArgs
		expectedErr string
		expected    *batchv1.Job
	}{{
		name:        "empty",
		args:        JobArgs{},
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
			got, err := NewJobOps(test.args)

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

func validCreateArgs() JobArgs {
	return JobArgs{
		UID:     schedulerUID,
		Image:   testImage,
		Action:  "create",
		JobName: validJobName,
		Secret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "key.json",
		},
		Owner:    reconcilertesting.NewScheduler(schedulerName, testNS),
		TopicID:  topicID,
		Schedule: onceAMinuteSchedule,
		Data:     testData,
	}
}

func validCreateJob() *batchv1.Job {
	owner := reconcilertesting.NewScheduler(schedulerName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            SchedulerJobName(owner, "create"),
			Namespace:       testNS,
			Labels:          SchedulerJobLabels(owner, "create"),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     podTemplate("create"),
		},
	}
}

func validDeleteArgs() JobArgs {
	return JobArgs{
		UID:     schedulerUID,
		Image:   testImage,
		Action:  "delete",
		JobName: validJobName,
		Secret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "key.json",
		},
		Owner: reconcilertesting.NewScheduler(schedulerName, testNS),
	}
}

func validDeleteJob() *batchv1.Job {
	owner := reconcilertesting.NewScheduler(schedulerName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            SchedulerJobName(owner, "delete"),
			Namespace:       testNS,
			Labels:          SchedulerJobLabels(owner, "delete"),
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
			Name:  "JOB_NAME",
			Value: validJobName,
		},
	}
	if action == "create" {
		createEnv := []corev1.EnvVar{
			{
				Name:  "JOB_PARENT",
				Value: validParent,
			}, {
				Name:  "PUBSUB_TOPIC_ID",
				Value: topicID,
			}, {
				Name:  "SCHEDULE",
				Value: onceAMinuteSchedule,
			}, {
				Name:  "DATA",
				Value: testData,
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
				"resource-uid": schedulerUID,
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
