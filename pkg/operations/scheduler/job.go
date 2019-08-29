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
	"regexp"
	"strings"

	"go.uber.org/zap"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"

	"github.com/google/knative-gcp/pkg/operations"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// TODO: Tighten up the matching here
	jobNameFormat = "projects/.*/locations/.*/jobs/.*"
)

// TODO: the job could output the resolved projectID.
type JobActionResult struct {
	// Result is the result the operation attempted.
	Result bool `json:"result,omitempty"`
	// Error is the error string if failure occurred
	Error string `json:"error,omitempty"`
	// JobName holds the name of the created job
	// and is filled in during create operation.
	JobName string `json:"jobName,omitempty"`
	// Project is the project id that we used (this might have
	// been defaulted, to we'll expose it).
	ProjectId string `json:"projectId,omitempty"`
}

// JobArgs are the configuration required to make a NewJobOps.
type JobArgs struct {
	// UID of the resource that caused the action to be taken. Will
	// be added as a label to the podtemplate.
	UID string

	// Image is the actual binary that we'll run to operate on the
	// notification.
	Image string

	// Action is what the binary should do
	Action string

	// TopicID we'll use for pubsub target.
	TopicID string

	// JobName is the name of the Scheduler Job that we're
	// operating on. The format is like so:
	// projects/PROJECT_ID/locations/LOCATION_ID/jobs/JobId
	JobName string

	// Schedule for the Job
	Schedule string

	// Data to send in the payload
	Data string

	Secret corev1.SecretKeySelector
	Owner  kmeta.OwnerRefable
}

// NewJobOps returns a new batch Job resource.
func NewJobOps(arg JobArgs) (*batchv1.Job, error) {
	if err := validateArgs(arg); err != nil {
		return nil, err
	}

	env := []corev1.EnvVar{{
		Name:  "ACTION",
		Value: arg.Action,
	}, {
		Name:  "JOB_NAME",
		Value: arg.JobName,
	}}

	switch arg.Action {
	case operations.ActionCreate:
		jobName := arg.JobName
		// JobName is like this:
		// projects/PROJECT_ID/locations/LOCATION_ID/jobs/JobId
		// For create we need a Parent, which (in the above is):
		// projects/PROJECT_ID/locations/LOCATION_ID
		// so construct it.
		parent := jobName[0:strings.LastIndex(jobName, "/jobs/")]

		env = append(env, []corev1.EnvVar{
			{
				Name:  "JOB_PARENT",
				Value: parent,
			}, {
				Name:  "PUBSUB_TOPIC_ID",
				Value: arg.TopicID,
			}, {
				Name:  "SCHEDULE",
				Value: arg.Schedule,
			}, {
				Name:  "DATA",
				Value: arg.Data,
			}}...)
	}

	podTemplate := operations.MakePodTemplate(arg.Image, arg.UID, arg.Action, arg.Secret, env...)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            SchedulerJobName(arg.Owner, arg.Action),
			Namespace:       arg.Owner.GetObjectMeta().GetNamespace(),
			Labels:          SchedulerJobLabels(arg.Owner, arg.Action),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(arg.Owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}, nil
}

// JobOps defines the configuration to use for this operation.
type JobOps struct {
	SchedulerOps

	// Action is the operation the job should run.
	// Options: [exists, create, delete]
	Action string `envconfig:"ACTION" required:"true"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"false"`

	// Schedule specification
	Schedule string `envconfig:"SCHEDULE" required:"false"`

	// JobName is the environment variable containing the name of the
	// job to operate on. F
	JobName string `envconfig:"JOB_NAME" required:"false" default:""`

	// Parent is the parent of the job.
	Parent string `envconfig:"JOB_PARENT" required:"false" default:""`

	// Data is the data to send in the payload.
	Data string `envconfig:"DATA" required:"false" default:""`
}

// Run will perform the action configured upon a subscription.
func (n *JobOps) Run(ctx context.Context) error {
	if n.client == nil {
		return errors.New("pub/sub client is nil")
	}
	logger := logging.FromContext(ctx)

	logger = logger.With(
		zap.String("action", n.Action),
		zap.String("project", n.Project),
		zap.String("jobName", n.JobName),
	)

	logger.Info("Scheduler Job Job.")

	switch n.Action {
	case operations.ActionExists:
		// If notification doesn't exist, that is an error.
		logger.Info("Previously created.")

	case operations.ActionCreate:
		logger.Info("Creating job %q", n.JobName)

		j, err := n.client.CreateJob(ctx, &schedulerpb.CreateJobRequest{
			Parent: n.Parent,
			Job: &schedulerpb.Job{
				Name: n.JobName,
				Target: &schedulerpb.Job_PubsubTarget{
					PubsubTarget: &schedulerpb.PubsubTarget{
						TopicName: fmt.Sprintf("projects/%s/topics/%s", n.Project, n.Topic),
						Data:      []byte(n.Data),
					},
				},
				Schedule: n.Schedule,
			},
		})
		if err != nil {
			logger.Infof("Failed to create Job %q: %s", n.JobName, err)
			result := &JobActionResult{
				Result: false,
				Error:  err.Error(),
			}
			writeErr := n.writeTerminationMessage(result)
			if writeErr != nil {
				logger.Infof("Failed to write termination message: %s", writeErr)
			}
			return err
		}
		result := &JobActionResult{
			Result:  true,
			JobName: n.JobName,
		}
		writeErr := n.writeTerminationMessage(result)
		if writeErr != nil {
			logger.Infof("Failed to write termination message: %s", writeErr)
		}
		logger.Infof("Created Job: %+v", j)
	case operations.ActionDelete:
		logger.Info("Deleting job %q", n.JobName)
		err := n.client.DeleteJob(ctx, &schedulerpb.DeleteJobRequest{Name: n.JobName})
		if err == nil {
			result := &JobActionResult{
				Result:  true,
				JobName: n.JobName,
			}
			writeErr := n.writeTerminationMessage(result)
			if writeErr != nil {
				logger.Infof("Failed to write termination message: %s", writeErr)
			}
			logger.Infof("Deleted Job: %q", n.JobName)
			logger.Info("Done.")
			return nil
		}

		if st, ok := gstatus.FromError(err); !ok {
			logger.Infof("error from the cloud scheduler client: %s", err)
			writeErr := n.writeTerminationMessage(&JobActionResult{Result: false, Error: err.Error()})
			if writeErr != nil {
				logger.Infof("Failed to write termination message: %s", writeErr)
				return err
			}
			return err
		} else if st.Code() != codes.NotFound {
			writeErr := n.writeTerminationMessage(&JobActionResult{Result: false, Error: err.Error()})
			if writeErr != nil {
				logger.Infof("Failed to write termination message: %s", writeErr)
				return err
			}
			return err
		}
		logger.Infof("The job %q does not exist, calling it good...", n.JobName)
		writeErr := n.writeTerminationMessage(&JobActionResult{Result: true, JobName: n.JobName})
		if writeErr != nil {
			logger.Infof("Failed to write termination message: %s", writeErr)
			return err
		}
	default:
		return fmt.Errorf("unknown action value %v", n.Action)
	}

	logger.Info("Done.")
	return nil
}

func (n *JobOps) writeTerminationMessage(result *JobActionResult) error {
	// Always add the project regardless of what we did.
	result.ProjectId = n.Project
	m, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", m, 0644)
}

func validateArgs(arg JobArgs) error {
	if arg.UID == "" {
		return fmt.Errorf("missing UID")
	}
	if arg.Image == "" {
		return fmt.Errorf("missing Image")
	}
	if arg.Action == "" {
		return fmt.Errorf("missing Action")
	}
	if arg.JobName == "" {
		return fmt.Errorf("missing JobName")
	}
	match, err := regexp.Match(jobNameFormat, []byte(arg.JobName))
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("JobName format is wrong")
	}
	if arg.Secret.Name == "" || arg.Secret.Key == "" {
		return fmt.Errorf("invalid secret missing name or key")
	}
	if arg.Owner == nil {
		return fmt.Errorf("missing owner")
	}

	switch arg.Action {
	case operations.ActionCreate:
		if arg.TopicID == "" {
			return fmt.Errorf("missing TopicID")
		}
		if arg.Schedule == "" {
			return fmt.Errorf("missing Schedule")
		}
		if arg.Data == "" {
			return fmt.Errorf("missing Data")
		}

	}
	return nil
}
