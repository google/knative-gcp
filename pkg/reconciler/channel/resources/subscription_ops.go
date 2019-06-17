/*
Copyright 2019 Google LLC

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

package resources

import (
	"fmt"
	"github.com/knative/pkg/kmeta"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
)

const (
	ActionCreate = "create"
	ActionDelete = "delete"
)

func JobLabels(name, action string) map[string]string {
	return map[string]string{
		"pubsub.cloud.run/channel": "pullsub-" + name + "-" + action,
	}
}

type Args struct {
	Image          string
	Action         string
	ProjectID      string
	TopicID        string
	SubscriptionID string
	Source         *v1alpha1.Channel
}

func NewSubscriptionOps(args Args) *batchv1.Job {
	podTemplate := makePodTemplate(args)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "pubsub-" + args.Source.GetName() + "-" + args.Action + "-",
			Namespace:       args.Source.GetNamespace(),
			Labels:          JobLabels(args.Source.GetName(), args.Action),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Source)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}
}

func IsJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsJobSucceeded(job *batchv1.Job) bool {
	return !IsJobFailed(job)
}

func IsJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func JobFailedMessage(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return fmt.Sprintf("[%s] %s", c.Reason, c.Message)
		}
	}
	return ""
}

func GetFirstTerminationMessage(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
			return cs.State.Terminated.Message
		}
	}
	return ""
}

// makePodTemplate creates a pod template for a Job.
func makePodTemplate(opt Args, extEnv ...corev1.EnvVar) *corev1.PodTemplateSpec {
	env := []corev1.EnvVar{{
		Name:  "ACTION",
		Value: opt.Action,
	}, {
		Name:  "PROJECT_ID",
		Value: opt.ProjectID,
	}, {
		Name:  "PUBSUB_TOPIC_ID",
		Value: opt.TopicID,
	}, {
		Name:  "PUBSUB_SUBSCRIPTION_ID",
		Value: opt.SubscriptionID,
	}}

	if len(extEnv) > 0 {
		env = append(env, extEnv...)
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "job",
				Image:           opt.Image,
				ImagePullPolicy: "Always",
				Env:             env,
			}},
		},
	}
}
