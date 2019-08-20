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

package pubsub

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"

	ops "github.com/google/knative-gcp/pkg/operations"
	operations "github.com/google/knative-gcp/pkg/operations/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "PubSub"
)

// Reconciler implements controller.Reconciler for Channel resources.
type PubSubBase struct {
	*reconciler.Base

	TopicOpsImage        string
	SubscriptionOpsImage string
}

func (c *PubSubBase) EnsureSubscriptionExists(ctx context.Context, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, topic, subscription string) (ops.OpsJobStatus, error) {
	return c.ensureSubscriptionJob(ctx, operations.SubArgs{
		Image:          c.SubscriptionOpsImage,
		Action:         ops.ActionExists,
		ProjectID:      project,
		TopicID:        topic,
		SubscriptionID: subscription,
		Secret:         secret,
		Owner:          owner,
	})
}

func (c *PubSubBase) EnsureSubscriptionCreated(ctx context.Context, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, topic, subscription string, ackDeadline time.Duration, retainAcked bool, retainDuration time.Duration) (ops.OpsJobStatus, error) {
	return c.ensureSubscriptionJob(ctx, operations.SubArgs{
		Image:               c.SubscriptionOpsImage,
		Action:              ops.ActionCreate,
		ProjectID:           project,
		TopicID:             topic,
		SubscriptionID:      subscription,
		AckDeadline:         ackDeadline,
		RetainAckedMessages: retainAcked,
		RetentionDuration:   retainDuration,
		Secret:              secret,
		Owner:               owner,
	})
}

func (c *PubSubBase) EnsureSubscriptionDeleted(ctx context.Context, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, topic, subscription string) (ops.OpsJobStatus, error) {
	return c.ensureSubscriptionJob(ctx, operations.SubArgs{
		Image:          c.SubscriptionOpsImage,
		Action:         ops.ActionDelete,
		ProjectID:      project,
		TopicID:        topic,
		SubscriptionID: subscription,
		Secret:         secret,
		Owner:          owner,
	})
}

func (c *PubSubBase) EnsureTopicExists(ctx context.Context, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, topic string) (ops.OpsJobStatus, error) {
	return c.ensureTopicJob(ctx, operations.TopicArgs{
		Image:     c.TopicOpsImage,
		Action:    ops.ActionExists,
		ProjectID: project,
		TopicID:   topic,
		Secret:    secret,
		Owner:     owner,
	})
}

func (c *PubSubBase) EnsureTopicCreated(ctx context.Context, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, topic string) (ops.OpsJobStatus, error) {
	return c.ensureTopicJob(ctx, operations.TopicArgs{
		Image:     c.TopicOpsImage,
		Action:    ops.ActionCreate,
		ProjectID: project,
		TopicID:   topic,
		Secret:    secret,
		Owner:     owner,
	})
}

func (c *PubSubBase) EnsureTopicDeleted(ctx context.Context, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, topic string) (ops.OpsJobStatus, error) {
	return c.ensureTopicJob(ctx, operations.TopicArgs{
		Image:     c.TopicOpsImage,
		Action:    ops.ActionDelete,
		ProjectID: project,
		TopicID:   topic,
		Secret:    secret,
		Owner:     owner,
	})
}

func (c *PubSubBase) ensureTopicJob(ctx context.Context, args operations.TopicArgs) (ops.OpsJobStatus, error) {
	job, err := c.getJob(ctx, args.Owner.GetObjectMeta(), labels.SelectorFromSet(operations.TopicJobLabels(args.Owner, args.Action)))
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating.")

		job = operations.NewTopicOps(args)

		job, err := c.KubeClientSet.BatchV1().Jobs(args.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return ops.OpsJobCreateFailed, nil
		}

		c.Logger.Debugw("Created Job.")
		return ops.OpsJobCreated, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return ops.OpsJobGetFailed, err
	}

	if ops.IsJobComplete(job) {
		c.Logger.Debugw("Job is complete.")
		if ops.IsJobSucceeded(job) {
			return ops.OpsJobCompleteSuccessful, nil
		} else if ops.IsJobFailed(job) {
			return ops.OpsJobCompleteFailed, errors.New(ops.JobFailedMessage(job))
		}
	}
	c.Logger.Debug("Job still active.", zap.Any("job", job))
	return ops.OpsJobOngoing, nil
}

func (c *PubSubBase) ensureSubscriptionJob(ctx context.Context, args operations.SubArgs) (ops.OpsJobStatus, error) {
	job, err := c.getJob(ctx, args.Owner.GetObjectMeta(), labels.SelectorFromSet(operations.SubscriptionJobLabels(args.Owner, args.Action)))
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating.")

		args.Image = c.SubscriptionOpsImage

		job = operations.NewSubscriptionOps(args)

		job, err := c.KubeClientSet.BatchV1().Jobs(args.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return ops.OpsJobCreateFailed, nil
		}

		c.Logger.Debugw("Created Job.")
		return ops.OpsJobCreated, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return ops.OpsJobGetFailed, err
	}

	if ops.IsJobComplete(job) {
		c.Logger.Debugw("Job is complete.")
		if ops.IsJobSucceeded(job) {
			return ops.OpsJobCompleteSuccessful, nil
		} else if ops.IsJobFailed(job) {
			return ops.OpsJobCompleteFailed, errors.New(ops.JobFailedMessage(job))
		}
	}
	c.Logger.Debug("Job still active.", zap.Any("job", job))
	return ops.OpsJobOngoing, nil
}

func (r *PubSubBase) getJob(ctx context.Context, owner metav1.Object, ls labels.Selector) (*v1.Job, error) {
	list, err := r.KubeClientSet.BatchV1().Jobs(owner.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: ls.String(),
	})
	if err != nil {
		return nil, err
	}

	for _, i := range list.Items {
		if metav1.IsControlledBy(&i, owner) {
			return &i, nil
		}
	}

	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}
