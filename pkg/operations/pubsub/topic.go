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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/operations"
)

// TODO: This is currently only used on success to communicate the
// project status. If there's something else that could be useful
// to communicate to the controller, add them here.
type TopicActionResult struct {
	// Project is the project id that we used (this might have
	// been defaulted, so we'll expose it so that controller can
	// reflect this in the Status).
	ProjectId string `json:"projectId,omitempty"`

	// Reason is the reason of the result.
	Reason string `json:"reason,omitempty"`
}

type TopicArgs struct {
	UID       string
	Image     string
	Action    string
	ProjectID string
	TopicID   string
	Secret    corev1.SecretKeySelector
	Owner     kmeta.OwnerRefable
}

func NewTopicOps(arg TopicArgs) *batchv1.Job {
	podTemplate := operations.MakePodTemplate(arg.Image, arg.UID, arg.Action, arg.Secret, []corev1.EnvVar{{
		Name:  "ACTION",
		Value: arg.Action,
	}, {
		Name:  "PROJECT_ID",
		Value: arg.ProjectID,
	}, {
		Name:  "PUBSUB_TOPIC_ID",
		Value: arg.TopicID,
	}}...)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            TopicJobName(arg.Owner, arg.Action),
			Namespace:       arg.Owner.GetObjectMeta().GetNamespace(),
			Labels:          TopicJobLabels(arg.Owner, arg.Action),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(arg.Owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}
}

type TopicOps struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// Action is the operation the job should run.
	// Options: [exists, create, delete]
	Action string `envconfig:"ACTION" required:"true"`

	// Topic is the environment variable containing the PubSub Topic being
	// created. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`
}

func (t *TopicOps) Run(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	if t.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			return t.terminationErr(logger, fmt.Errorf("failed to find project id, %s", err))
		}
		t.Project = project
	}

	logger = logger.With(
		zap.String("action", t.Action),
		zap.String("project", t.Project),
		zap.String("topic", t.Topic),
	)

	logger.Info("Pub/Sub Topic Job.")

	client, err := pubsub.NewClient(ctx, t.Project)
	if err != nil {
		return t.terminationErr(logger, fmt.Errorf("failed to create Pub/Sub client, %s", err))
	}

	topic := client.Topic(t.Topic)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return t.terminationErr(logger, fmt.Errorf("failed to verify topic exists, %s", err))
	}

	switch t.Action {
	case operations.ActionExists:
		// If topic doesn't exist, that is an error.
		if !exists {
			return t.terminationErr(logger, errors.New("topic does not exist"))
		}
		logger.Info("Previously created.")

	case operations.ActionCreate:
		// If topic doesn't exist, create it.
		if !exists {
			// Create a new topic with the given name.
			topic, err = client.CreateTopic(ctx, t.Topic)
			if err != nil {
				return t.terminationErr(logger, fmt.Errorf("failed to create topic, %s", err))
			}
			logger.Info("Successfully created.")
		} else {
			// TODO: here is where we could update topic config.
			logger.Info("Previously created.")
		}
	case operations.ActionDelete:
		if exists {
			if err := topic.Delete(ctx); err != nil {
				return t.terminationErr(logger, fmt.Errorf("failed to delete topic, %s", err))
			}
			logger.Info("Successfully deleted.")
		} else {
			logger.Info("Previously deleted.")
		}

	default:
		return t.terminationErr(logger, fmt.Errorf("unknown action value %s", t.Action))
	}

	t.writeTerminationMessage(&TopicActionResult{})
	logger.Info("Done.")
	return nil
}

func (t *TopicOps) terminationErr(logger *zap.SugaredLogger, e error) error {
	result := &TopicActionResult{Reason: e.Error()}
	if err := t.writeTerminationMessage(result); err != nil {
		logger.Errorf("Failed to write termination log", zap.Error(err))
	}
	return e
}

func (t *TopicOps) writeTerminationMessage(result *TopicActionResult) error {
	// Always add the project regardless of what we did.
	result.ProjectId = t.Project
	m, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", m, 0644)
}
