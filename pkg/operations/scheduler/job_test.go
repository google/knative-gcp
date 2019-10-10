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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/operations"
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
		args        operations.JobArgs
		expectedErr string
	}{{
		name: "missing JobName",
		args: func(a SchedulerJobCreateArgs) SchedulerJobCreateArgs {
			a.JobName = ""
			return a
		}(validCreateArgs()),
		expectedErr: "missing JobName",
	}, {
		name: "invalid JobName format",
		args: func(a SchedulerJobCreateArgs) SchedulerJobCreateArgs {
			a.JobName = "projects/foo"
			return a
		}(validCreateArgs()),
		expectedErr: "JobName format is wrong",
	}, {
		name: "missing topicId",
		args: func(a SchedulerJobCreateArgs) SchedulerJobCreateArgs {
			a.TopicID = ""
			return a
		}(validCreateArgs()),
		expectedErr: "missing TopicID",
	}, {
		name: "missing Schedule",
		args: func(a SchedulerJobCreateArgs) SchedulerJobCreateArgs {
			a.Schedule = ""
			return a
		}(validCreateArgs()),
		expectedErr: "missing Schedule",
	}, {
		name: "missing data",
		args: func(a SchedulerJobCreateArgs) SchedulerJobCreateArgs {
			a.Data = ""
			return a
		}(validCreateArgs()),
		expectedErr: "missing Data",
	}, {
		name:        "valid create",
		args:        validCreateArgs(),
		expectedErr: "",
	}, {
		name:        "valid delete",
		args:        validDeleteArgs(),
		expectedErr: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.args.Validate()

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
		opCtx       operations.OpCtx
		args        operations.JobArgs
		expectedErr string
		expected    *batchv1.Job
	}{{
		name: "empty",
		args: func(a SchedulerJobCreateArgs) SchedulerJobCreateArgs {
			a.JobName = ""
			return a
		}(validCreateArgs()),
		expectedErr: "missing JobName",
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

	opCtx := operations.OpCtx{
		UID:   schedulerUID,
		Image: testImage,
		Secret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "key.json",
		},
		Owner: reconcilertesting.NewScheduler(schedulerName, testNS),
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

func validCreateArgs() SchedulerJobCreateArgs {
	return SchedulerJobCreateArgs{
		SchedulerJobArgs: SchedulerJobArgs{
			JobName: validJobName,
		},
		TopicID:  topicID,
		Schedule: onceAMinuteSchedule,
		Data:     testData,
	}
}

func validCreateJob() *batchv1.Job {
	owner := reconcilertesting.NewScheduler(schedulerName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("scheduler-j-%s-scheduler-create", strings.ToLower(schedulerName)),
			Namespace: testNS,
			Labels: map[string]string{
				"events.cloud.run/scheduler-job": fmt.Sprintf("%s-Scheduler-createops", schedulerName),
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

func validDeleteArgs() SchedulerJobDeleteArgs {
	return SchedulerJobDeleteArgs{
		SchedulerJobArgs{
			JobName: validJobName,
		},
	}
}

func validDeleteJob() *batchv1.Job {
	owner := reconcilertesting.NewScheduler(schedulerName, testNS)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("scheduler-j-%s-scheduler-delete", strings.ToLower(schedulerName)),
			Namespace: testNS,
			Labels: map[string]string{
				"events.cloud.run/scheduler-job": fmt.Sprintf("%s-Scheduler-deleteops", schedulerName),
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
