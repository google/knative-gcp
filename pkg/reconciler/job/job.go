/*
Copyright 2019 Google LLC.

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

package job

import (
	"context"
	"errors"

	"github.com/google/knative-gcp/pkg/operations"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
)

type Reconciler struct {
	KubeClientSet kubernetes.Interface
	JobLister     batchv1listers.JobLister
	Logger        *zap.SugaredLogger
}

func (c *Reconciler) EnsureOpJob(ctx context.Context, opCtx operations.OpCtx, args operations.JobArgs) (operations.OpsJobStatus, error) {
	jobName := operations.JobName(opCtx, args)
	job, err := c.JobLister.Jobs(opCtx.Owner.GetObjectMeta().GetNamespace()).Get(jobName)

	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating with:", zap.Any("opCtx", opCtx), zap.Any("args", args))

		job, err = operations.NewOpsJob(opCtx, args)
		if err != nil {
			c.Logger.Debugw("Invalid job args.", zap.Any("opCtx", opCtx), zap.Any("args", args), zap.Error(err))
			return operations.OpsJobCreateFailed, err
		}

		job, err := c.KubeClientSet.BatchV1().Jobs(opCtx.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return operations.OpsJobCreateFailed, nil
		}

		c.Logger.Debugw("Created Job.")
		return operations.OpsJobCreated, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return operations.OpsJobGetFailed, err
		// TODO: Handle this case
		//	} else if !metav1.IsControlledBy(job, opCtx.Owner) {
		//		return operations.OpsJobCreateFailed, fmt.Errorf()
	}

	if operations.IsJobComplete(job) {
		c.Logger.Debugw("Job is complete.")
		if operations.IsJobSucceeded(job) {
			return operations.OpsJobCompleteSuccessful, nil
		} else if operations.IsJobFailed(job) {
			return operations.OpsJobCompleteFailed, errors.New(operations.JobFailedMessage(job))
		}
	}
	c.Logger.Debug("Job still active.", zap.Any("job", job))
	return operations.OpsJobOngoing, nil
}
