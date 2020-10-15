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

func IsJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
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
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		logger.Infof("Found pod: %q", pod.Name)
		return &pod, nil
	}
	return nil, fmt.Errorf("pod not found")
}
