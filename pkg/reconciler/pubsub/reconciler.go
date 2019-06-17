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

	"github.com/knative/pkg/kmeta"
	"go.uber.org/zap"
	v1 "k8s.io/api/batch/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/operations"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
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

func (c *PubSubBase) ensureTopic(ctx context.Context, channel *v1alpha1.Channel, topicID string) (bool, error) {
	return false, errors.New("not implemented")
}

type OpsState string

const (
	OpsGetFailedState         OpsState = "JOB_GET_FAILED"
	OpsCreatedState           OpsState = "JOB_CREATED"
	OpsCreateFailedState      OpsState = "JOB_CREATE_FAILED"
	OpsCompeteSuccessfulState OpsState = "JOB_SUCCESSFUL"
	OpsCompeteFailedState     OpsState = "JOB_FAILED"
	OpsOngoingState           OpsState = "JOB_ONGOING"
)

func (c *PubSubBase) EnsureSubscription(ctx context.Context, owner kmeta.OwnerRefable, project, topic, subscription string) (OpsState, error) {
	return c.ensureSubscriptionJob(ctx, operations.ActionCreate, owner, project, topic, subscription)
}

func (c *PubSubBase) EnsureSubscriptionDeleted(ctx context.Context, owner kmeta.OwnerRefable, project, topic, subscription string) (OpsState, error) {
	return c.ensureSubscriptionJob(ctx, operations.ActionDelete, owner, project, topic, subscription)
}

func (c *PubSubBase) EnsureTopic(ctx context.Context, owner kmeta.OwnerRefable, project, topic string) (OpsState, error) {
	return c.ensureTopicJob(ctx, operations.ActionCreate, owner, project, topic)
}

func (c *PubSubBase) EnsureTopicDeleted(ctx context.Context, owner kmeta.OwnerRefable, project, topic string) (OpsState, error) {
	return c.ensureTopicJob(ctx, operations.ActionDelete, owner, project, topic)
}

func (c *PubSubBase) ensureTopicJob(ctx context.Context, action string, owner kmeta.OwnerRefable, project, topic string) (OpsState, error) {
	job, err := c.getJob(ctx, owner.GetObjectMeta(), labels.SelectorFromSet(operations.TopicJobLabels(owner, action)))
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating.")

		job = operations.NewTopicOps(operations.TopicArgs{
			Image:     c.SubscriptionOpsImage,
			Action:    action,
			ProjectID: project,
			TopicID:   topic,
			Owner:     owner,
		})

		job, err := c.KubeClientSet.BatchV1().Jobs(owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return OpsCreateFailedState, nil
		}

		c.Logger.Debugw("Created Job.")
		return OpsCreatedState, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return OpsGetFailedState, err
	}

	if operations.IsJobComplete(job) {
		c.Logger.Debugw("Job is complete.")
		if operations.IsJobSucceeded(job) {
			return OpsCompeteSuccessfulState, nil
		} else if operations.IsJobFailed(job) {
			return OpsCompeteFailedState, errors.New(operations.JobFailedMessage(job))
		}
	}
	c.Logger.Debug("Job still active.", zap.Any("job", job))
	return OpsOngoingState, nil
}

func (c *PubSubBase) ensureSubscriptionJob(ctx context.Context, action string, owner kmeta.OwnerRefable, project, topic, subscription string) (OpsState, error) {
	job, err := c.getJob(ctx, owner.GetObjectMeta(), labels.SelectorFromSet(operations.SubscriptionJobLabels(owner, action)))
	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating.")

		job = operations.NewSubscriptionOps(operations.SubArgs{
			Image:          c.SubscriptionOpsImage,
			Action:         action,
			ProjectID:      project,
			TopicID:        topic,
			SubscriptionID: subscription,
			Owner:          owner,
		})

		job, err := c.KubeClientSet.BatchV1().Jobs(owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return OpsCreateFailedState, nil
		}

		c.Logger.Debugw("Created Job.")
		return OpsCreatedState, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return OpsGetFailedState, err
	}

	if operations.IsJobComplete(job) {
		c.Logger.Debugw("Job is complete.")
		if operations.IsJobSucceeded(job) {
			return OpsCompeteSuccessfulState, nil
		} else if operations.IsJobFailed(job) {
			return OpsCompeteFailedState, errors.New(operations.JobFailedMessage(job))
		}
	}
	c.Logger.Debug("Job still active.", zap.Any("job", job))
	return OpsOngoingState, nil
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
