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

package operations

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

type OpsJobStatus string

const (
	OpsJobGetFailed          OpsJobStatus = "JOB_GET_FAILED"
	OpsJobCreated            OpsJobStatus = "JOB_CREATED"
	OpsJobCreateFailed       OpsJobStatus = "JOB_CREATE_FAILED"
	OpsJobCompleteSuccessful OpsJobStatus = "JOB_SUCCESSFUL"
	OpsJobCompleteFailed     OpsJobStatus = "JOB_FAILED"
	OpsJobOngoing            OpsJobStatus = "JOB_ONGOING"
)

type JobArgs interface {
	// Group of operations, e.g. pubsub, storage, ...
	OperationGroup() string
	// Typically single character subgroup of the operation,
	// e.g. t for pubsub topic operations or n for storage
	// notification operations.
	OperationSubgroup() string
	// Action to be performed by the job.
	Action() string
	// Environment variables to set in the job.
	Env() []corev1.EnvVar
	// Label key identifying the job. Label value will consist of
	// owner name, owner GVK, and the operation action.
	LabelKey() string
	// Ensures that the job args are valid, returning an error
	// otherwise.
	Validate() error
}

type OpCtx struct {
	// Image that should be used when performing the operation.
	Image string
	// UID of the resource that caused this action to be taken. It
	// will be added to the job's pod template as a label as
	// "resource-uid".
	UID    string
	Secret corev1.SecretKeySelector
	Owner  kmeta.OwnerRefable
}

func JobName(o OpCtx, a JobArgs) string {
	base := strings.ToLower(
		strings.Join(append([]string{
			a.OperationGroup(),
			a.OperationSubgroup(),
			o.Owner.GetObjectMeta().GetName(),
			o.Owner.GetGroupVersionKind().Kind}),
			"-"),
	)
	return kmeta.ChildName(base, "-"+a.Action())
}

func JobLabelVal(o OpCtx, a JobArgs) (value string) {
	value = strings.Join([]string{
		o.Owner.GetObjectMeta().GetName(),
		o.Owner.GetGroupVersionKind().Kind,
		a.Action(),
	}, "-")
	value = kmeta.ChildName(value, "ops")
	return
}

func JobLabels(o OpCtx, a JobArgs) map[string]string {
	return map[string]string{
		"events.cloud.google.com/" + a.LabelKey(): JobLabelVal(o, a),
	}
}

func NewOpsJob(o OpCtx, a JobArgs) (*batchv1.Job, error) {
	if err := validateJobArgs(a); err != nil {
		return nil, err
	}

	podTemplate := MakePodTemplate(
		o.Image, o.UID, a.Action(), o.Secret,
		append(a.Env(), corev1.EnvVar{
			Name:  "ACTION",
			Value: a.Action(),
		})...)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            JobName(o, a),
			Namespace:       o.Owner.GetObjectMeta().GetNamespace(),
			Labels:          JobLabels(o, a),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(o.Owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}, nil
}

func validateJobArgs(a JobArgs) error {
	if a.Action() == "" {
		return fmt.Errorf("missing Action")
	}

	return a.Validate()
}

func validateOpCtx(o OpCtx) error {
	if o.UID == "" {
		return fmt.Errorf("missing UID")
	}
	if o.Image == "" {
		return fmt.Errorf("missing Image")
	}
	if o.Secret.Name == "" || o.Secret.Key == "" {
		return fmt.Errorf("invalid secret missing name or key")
	}
	if o.Owner == nil {
		return fmt.Errorf("missing owner")
	}
	return nil
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

// GetJobPod will find the Pod that belongs to the resource that created it.
// Uses label ""controller-uid  as the label selector. So, your job should
// tag the job with that label as the UID of the resource that's needing it.
// For example, if you create a storage object that requires us to create
// a notification for it, the controller should set the label on the
// Job responsible for creating the Notification for it with the label
// "controller-uid" set to the uid of the storage CR.
func GetJobPod(ctx context.Context, kubeClientset kubernetes.Interface, namespace, uid, operation string) (*corev1.Pod, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("Looking for Pod with UID: %q action: %q", uid, operation)
	matchLabels := map[string]string{
		"resource-uid": uid,
		"action":       operation,
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		logger.Infof("Found pod: %q", pod.Name)
		return &pod, nil
	}
	return nil, fmt.Errorf("Pod not found")
}

// GetJobPodByJobName will find the Pods that belong to that job. Each pod
// for a given job will have label called: "job-name" set to the job that
// it belongs to, so just filter by that.
func GetJobPodByJobName(ctx context.Context, kubeClientset kubernetes.Interface, namespace, jobName string) (*corev1.Pod, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("Looking for Pod with jobname: %q", jobName)
	matchLabels := map[string]string{
		"job-name": jobName,
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		logger.Infof("Found pod: %q", pod.Name)
		return &pod, nil
	}
	return nil, fmt.Errorf("Pod not found")
}
