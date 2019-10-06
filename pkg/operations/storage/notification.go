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
	"strings"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	storageClient "cloud.google.com/go/storage"
	"github.com/google/knative-gcp/pkg/operations"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"

	corev1 "k8s.io/api/core/v1"
)

var (
	// Mapping of the storage importer eventTypes to google storage types.
	storageEventTypes = map[string]string{
		"finalize":       "OBJECT_FINALIZE",
		"archive":        "OBJECT_ARCHIVE",
		"delete":         "OBJECT_DELETE",
		"metadataUpdate": "OBJECT_METADATA_UPDATE",
	}
)

type NotificationActionResult struct {
	// Result is the result the operation attempted.
	Result bool `json:"result,omitempty"`
	// Error is the error string if failure occurred
	Error string `json:"error,omitempty"`
	// NotificationId holds the notification ID for GCS
	// and is filled in during create operation.
	NotificationId string `json:"notificationId,omitempty"`
	// Project is the project id that we used (this might have
	// been defaulted, to we'll expose it).
	ProjectId string `json:"projectId,omitempty"`
}

// NotificationArgs are the configuration required to make a NewNotificationOps.
type NotificationArgs struct {
	StorageArgs
	// Bucket
	Bucket string
}

func (n NotificationArgs) OperationSubgroup() string {
	return "n"
}

func (n NotificationArgs) LabelKey() string {
	return "notification"
}

func NotificationEnv(n NotificationArgs) []corev1.EnvVar {
	return append(StorageEnv(n.StorageArgs),
		corev1.EnvVar{
			Name:  "BUCKET",
			Value: n.Bucket,
		})
}

func ValidateNotificationArgs(n NotificationArgs) error {
	if n.Bucket == "" {
		return fmt.Errorf("missing Bucket")
	}
	return ValidateStorageArgs(n.StorageArgs)
}

type NotificationCreateArgs struct {
	NotificationArgs
	// TopicID we'll use for pubsub target.
	TopicID string
	// EventTypes is an array of strings specifying which
	// event types we want the notification to fire on.
	EventTypes []string
	// ObjectNamePrefix is an optional filter (currently ignored).
	ObjectNamePrefix string
}

var _ operations.JobArgs = NotificationCreateArgs{}

func (n NotificationCreateArgs) Action() string {
	return operations.ActionCreate
}

func (n NotificationCreateArgs) Env() []corev1.EnvVar {
	return append(NotificationEnv(n.NotificationArgs),
		corev1.EnvVar{
			Name:  "EVENT_TYPES",
			Value: strings.Join(n.EventTypes, ":"),
		},
		corev1.EnvVar{
			Name:  "PUBSUB_TOPIC_ID",
			Value: n.TopicID,
		})
}

func (n NotificationCreateArgs) Validate() error {
	if n.ProjectID == "" {
		return fmt.Errorf("missing ProjectID")
	}
	if n.TopicID == "" {
		return fmt.Errorf("missing TopicID")
	}
	if len(n.EventTypes) == 0 {
		return fmt.Errorf("missing EventTypes")
	}
	return ValidateNotificationArgs(n.NotificationArgs)
}

type NotificationDeleteArgs struct {
	NotificationArgs
	// NotificationId is the notifification ID that GCS gives
	// back to us. We need that to delete it.
	NotificationId string
}

var _ operations.JobArgs = NotificationDeleteArgs{}

func (n NotificationDeleteArgs) Action() string {
	return operations.ActionDelete
}

func (n NotificationDeleteArgs) Env() []corev1.EnvVar {
	return append(NotificationEnv(n.NotificationArgs),
		corev1.EnvVar{
			Name:  "NOTIFICATION_ID",
			Value: n.NotificationId,
		})
}

func (n NotificationDeleteArgs) Validate() error {
	if n.NotificationId == "" {
		return fmt.Errorf("missing NotificationId")
	}
	return ValidateNotificationArgs(n.NotificationArgs)
}

// NotificationOps defines the configuration to use for this operation.
type NotificationOps struct {
	StorageOps

	// Action is the operation the job should run.
	// Options: [exists, create, delete]
	Action string `envconfig:"ACTION" required:"true"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"false"`

	// Bucket to operate on
	Bucket string `envconfig:"BUCKET" required:"true"`

	// NotificationId is the environment variable containing the name of the
	// subscription to use.
	NotificationId string `envconfig:"NOTIFICATION_ID" required:"false" default:""`

	// EventTypes is a : separated eventtypes, if omitted all will be used.
	// TODO: Look at native envconfig list support
	EventTypes string `envconfig:"EVENT_TYPES" required:"false" default:""`

	// ObjectNamePrefix is an optional filter for the GCS
	ObjectNamePrefix string `envconfig:"OBJECT_NAME_PREFIX" required:"false" default:""`
}

// Run will perform the action configured upon a subscription.
func (n *NotificationOps) Run(ctx context.Context) error {
	if n.Client == nil {
		return errors.New("GCS client is nil")
	}
	logger := logging.FromContext(ctx)

	logger = logger.With(
		zap.String("action", n.Action),
		zap.String("project", n.Project),
		zap.String("topic", n.Topic),
		zap.String("subscription", n.NotificationId),
	)

	logger.Info("Storage Notification Job.")

	// Load the Bucket.
	bucket := n.Client.Bucket(n.Bucket)

	switch n.Action {
	case operations.ActionExists:
		// If notification doesn't exist, that is an error.
		logger.Info("Previously created.")

	case operations.ActionCreate:
		if n.Topic == "" {
			result := &NotificationActionResult{
				Result: false,
				Error:  "Topic not specified, need it for create",
			}
			writeErr := n.writeTerminationMessage(result)
			logger.Errorf("Failed to write termination message: %", writeErr)
			return fmt.Errorf("Topic not specified, need it for create")
		}
		customAttributes := make(map[string]string)

		// Add our own event type here...
		customAttributes["knative-gcp"] = "google.storage"

		eventTypes := strings.Split(n.EventTypes, ":")
		logger.Infof("Creating a notification on bucket %s", n.Bucket)

		nc := storageClient.Notification{
			TopicProjectID:   n.Project,
			TopicID:          n.Topic,
			PayloadFormat:    storageClient.JSONPayload,
			EventTypes:       n.toStorageEventTypes(eventTypes),
			ObjectNamePrefix: n.ObjectNamePrefix,
			CustomAttributes: customAttributes,
		}

		notification, err := bucket.AddNotification(ctx, &nc)
		if err != nil {
			result := &NotificationActionResult{
				Result: false,
				Error:  err.Error(),
			}
			logger.Infof("Failed to create Notification: %s", err)
			err = n.writeTerminationMessage(result)
			return err
		}
		logger.Infof("Created Notification %q", notification.ID)
		result := &NotificationActionResult{
			Result:         true,
			NotificationId: notification.ID,
		}
		err = n.writeTerminationMessage(result)
		if err != nil {
			logger.Infof("Failed to write termination message: %s", err)
			return err
		}
	case operations.ActionDelete:
		notifications, err := bucket.Notifications(ctx)
		if err != nil {
			logger.Infof("Failed to fetch existing notifications: %s", err)
			return err
		}

		// This is bit wonky because, we could always just try to delete, but figuring out
		// if an error returned is NotFound seems to not really work, so, we'll try
		// checking first the list and only then deleting.
		notificationId := n.NotificationId
		if notificationId != "" {
			if existing, ok := notifications[notificationId]; ok {
				logger.Infof("Found existing notification: %+v", existing)
				logger.Infof("Deleting notification as: %q", notificationId)
				err = bucket.DeleteNotification(ctx, notificationId)
				if err == nil {
					logger.Infof("Deleted Notification: %q", notificationId)
					err = n.writeTerminationMessage(&NotificationActionResult{Result: true})
					if err != nil {
						logger.Infof("Failed to write termination message: %s", err)
						return err
					}
					return nil
				}

				if st, ok := gstatus.FromError(err); !ok {
					logger.Infof("error from the cloud storage client: %s", err)
					writeErr := n.writeTerminationMessage(&NotificationActionResult{Result: false, Error: err.Error()})
					if writeErr != nil {
						logger.Infof("Failed to write termination message: %s", writeErr)
						return err
					}
					return err
				} else if st.Code() != codes.NotFound {
					writeErr := n.writeTerminationMessage(&NotificationActionResult{Result: false, Error: err.Error()})
					if writeErr != nil {
						logger.Infof("Failed to write termination message: %s", writeErr)
						return err
					}
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unknown action value %v", n.Action)
	}

	logger.Info("Done.")
	return nil
}

func (n *NotificationOps) toStorageEventTypes(eventTypes []string) []string {
	storageTypes := make([]string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		storageTypes = append(storageTypes, storageEventTypes[eventType])
	}
	return storageTypes
}

func (n *NotificationOps) writeTerminationMessage(result *NotificationActionResult) error {
	// Always add the project regardless of what we did.
	result.ProjectId = n.Project
	m, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", m, 0644)
}
