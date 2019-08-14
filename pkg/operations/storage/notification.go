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
	"errors"
	"fmt"
	//	"strconv"
	"io/ioutil"
	"strings"

	//	"github.com/google/go-cmp/cmp"
	//	"github.com/google/go-cmp/cmp/cmpopts"

	"go.uber.org/zap"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	storageClient "cloud.google.com/go/storage"
	"github.com/google/knative-gcp/pkg/operations"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// NotificationArgs are the configuration required to make a NewNotificationOps.
type NotificationArgs struct {
	// Image is the actual binary that we'll run to operate on the
	// notification.
	Image string
	// Action is what the binary should do
	Action    string
	ProjectID string
	// Bucket
	Bucket string
	// Topic we'll use for pubsub target.
	TopicID string
	// NotificationId is the notifification ID that GCS gives
	// back to us. We need that to delete it.
	NotificationId string
	// EventTypes is an array of strings specifying which
	// event types we want the notification to fire on.
	EventTypes []string
	// ObjectNamePrefix is an optional filter
	ObjectNamePrefix string
	// CustomAttributes is the list of additional attributes to have
	// GCS supply back to us when it sends a notification.
	CustomAttributes map[string]string
	Secret           corev1.SecretKeySelector
	Owner            kmeta.OwnerRefable
}

// NewNotificationOps returns a new batch Job resource.
func NewNotificationOps(arg NotificationArgs) *batchv1.Job {
	env := []corev1.EnvVar{{
		Name:  "ACTION",
		Value: arg.Action,
	}, {
		Name:  "PROJECT_ID",
		Value: arg.ProjectID,
	}, {
		Name:  "BUCKET",
		Value: arg.Bucket,
	}, {
		Name:  "PUBSUB_TOPIC_ID",
		Value: arg.TopicID,
	}}

	switch arg.Action {
	case operations.ActionCreate:
		env = append(env, []corev1.EnvVar{{
			Name:  "EVENT_TYPES",
			Value: strings.Join(arg.EventTypes, ":"),
		}}...)
	}

	podTemplate := operations.MakePodTemplate(arg.Image, arg.Secret, env...)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    NotificationJobName(arg.Owner, arg.Action),
			Namespace:       arg.Owner.GetObjectMeta().GetNamespace(),
			Labels:          NotificationJobLabels(arg.Owner, arg.Action),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(arg.Owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}
}

// TODO: the job could output the resolved projectID.

// NotificationOps defines the configuration to use for this operation.
type NotificationOps struct {
	StorageOps

	// Action is the operation the job should run.
	// Options: [exists, create, delete]
	Action string `envconfig:"ACTION" required:"true"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`

	// Bucket to operate on
	Bucket string `envconfig:"BUCKET" required:"true"`

	// Notification is the environment variable containing the name of the
	// subscription to use.
	Notification string `envconfig:"NOTIFICATION_ID" required:"false" default:""`

	// EventTypes is a : separated eventtypes, if omitted all will be used.
	EventTypes string `envconfig:"EVENT_TYPES" required:"false" default:""`

	// ObjectNamePrefix is an optional filter for the GCS
	ObjectNamePrefix string `envconfig:"OBJECT_NAME_PREFIX" required:"false" default:""`
}

//var (
//	ignoreNotificationConfig = cmpopts.IgnoreFields(pubsub.NotificationConfig{}, "Topic", "Labels")
//)

// Run will perform the action configured upon a subscription.
func (n *NotificationOps) Run(ctx context.Context) error {
	if n.Client == nil {
		return errors.New("pub/sub client is nil")
	}
	logger := logging.FromContext(ctx)

	logger = logger.With(
		zap.String("action", n.Action),
		zap.String("project", n.Project),
		zap.String("topic", n.Topic),
		zap.String("subscription", n.Notification),
	)

	logger.Info("Storage Notification Job.")

	// Load the Bucket.
	bucket := n.Client.Bucket(n.Bucket)

	/*
		notifications, err := bucket.Notifications(ctx)
		if err != nil {
			logger.Infof("Failed to fetch existing notifications: %s", err)
			return err
		}
		sub := s.Client.Notification(s.Notification)
		exists, err := sub.Exists(ctx)
		if err != nil {
			return fmt.Errorf("failed to verify notification exists: %s", err)
		}
	*/

	switch n.Action {
	case operations.ActionExists:
		// If notification doesn't exist, that is an error.
		logger.Info("Previously created.")

	case operations.ActionCreate:
		customAttributes := make(map[string]string)
		//		for k, v := range storage.Spec.CustomAttributes {
		//			customAttributes[k] = v
		//		}

		// Add our own event type here...
		customAttributes["knative-gcp"] = "google.storage"
		customAttributes["fromtemplate"] = "vaikas-was-here"

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

		logger.Info("NOTIFIcATION IS: %+v", nc)
		notification, err := bucket.AddNotification(ctx, &nc)

		/*
			notification, err := bucket.AddNotification(ctx, &storageClient.Notification{
				TopicProjectID:   n.Project,
				TopicID:          n.Topic,
				PayloadFormat:    storageClient.JSONPayload,
				EventTypes:       n.toStorageEventTypes(eventTypes),
				ObjectNamePrefix: n.ObjectNamePrefix,
				CustomAttributes: customAttributes,
			})
		*/
		if err != nil {
			logger.Infof("Failed to create Notification: %s", err)
			return err
		}
		logger.Infof("Created Notification %q", notification.ID)
		err = n.writeTerminationMessage([]byte(notification.ID))
		if err != nil {
			logger.Infof("Failed to write termination message: %s", err)
			return err
		}
		/*
			// subConfig is the wanted config based on settings.
			subConfig := pubsub.NotificationConfig{
				Topic:               topic,
				AckDeadline:         s.AckDeadline,
				RetainAckedMessages: s.RetainAckedMessages,
				RetentionDuration:   s.RetentionDuration,
			}

			// If topic doesn't exist, create it.
			if !exists {
				// Create a new subscription to the previous topic with the given name.
				sub, err = s.Client.CreateNotification(ctx, s.Notification, subConfig)
				if err != nil {
					return fmt.Errorf("failed to create subscription, %s", err)
				}
				logger.Info("Successfully created.")
			} else {
				// TODO: here is where we could update config.
				logger.Info("Previously created.")
				// Get current config.
				currentConfig, err := sub.Config(ctx)
				if err != nil {
					return fmt.Errorf("failed to get subscription config, %s", err)
				}
				// Compare the current config to the expected config. Update if different.
				if diff := cmp.Diff(subConfig, currentConfig, ignoreSubConfig); diff != "" {
					_, err := sub.Update(ctx, pubsub.NotificationConfig{
						AckDeadline:         s.AckDeadline,
						RetainAckedMessages: s.RetainAckedMessages,
						RetentionDuration:   s.RetentionDuration,
						Labels:              currentConfig.Labels,
					})
					if err != nil {
						return fmt.Errorf("failed to update subscription config, %s", err)
					}
					logger.Info("Updated subscription config.", zap.String("diff", diff))

				}
			}
		*/

	case operations.ActionDelete:
		logger.Info("Deleting")
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

	if len(storageTypes) == 0 {
		return append(storageTypes, "OBJECT_FINALIZE")
	}
	return storageTypes
}

func (n *NotificationOps) writeTerminationMessage(msg []byte) error {
	return ioutil.WriteFile("/dev/termination-log", msg, 0644)
}
