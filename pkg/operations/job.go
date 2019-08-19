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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
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

// GetJobProd will find the Pod that belongs to the resource that created it.
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
