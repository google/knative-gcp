/*
Copyright 2021 Google LLC

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

package celltenant

import (
	"context"
	"fmt"
	"time"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"

	"k8s.io/client-go/tools/record"

	"github.com/rickb777/date/period"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/logging"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	defaultMinimumBackoff = 1 * time.Second
	// Default maximum backoff duration used in the backoff retry policy for
	// pubsub subscriptions. 600 seconds is the longest supported time.
	defaultMaximumBackoff = 600 * time.Second
)

// TargetReconciler implements controller.Reconciler for CellTenant Targets.
type TargetReconciler struct {
	ProjectID string

	// pubsubClient is used as the Pubsub client when present.
	PubsubClient *pubsub.Client

	DataresidencyStore *dataresidency.Store

	// ClusterRegion is the region where GKE is running.
	ClusterRegion string
}

func (r *TargetReconciler) ReconcileRetryTopicAndSubscription(ctx context.Context, recorder record.EventRecorder, t Target) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling retry topic")
	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		t.StatusUpdater().MarkTopicUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		t.StatusUpdater().MarkSubscriptionUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}
	t.SetStatusProjectID(projectID)

	r.ClusterRegion, err = utils.ClusterRegion(r.ClusterRegion, metadataClient.NewDefaultMetadataClient)
	if err != nil {
		logger.Error("Failed to get cluster region: ", zap.Error(err))
		return err
	}

	client, err := r.getClientOrCreateNew(ctx, projectID, t.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}

	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, recorder)

	// Check if topic exists, and if not, create it.
	topicID := t.GetTopicID()
	topicConfig := &pubsub.TopicConfig{Labels: t.GetLabels()}
	if r.DataresidencyStore != nil {
		if r.DataresidencyStore.Load().DataResidencyDefaults.ComputeAllowedPersistenceRegions(topicConfig, r.ClusterRegion) {
			logging.FromContext(ctx).Debug("Updated Topic Config AllowedPersistenceRegions for Trigger", zap.Any("topicConfig", *topicConfig))
		}
	}
	topic, err := pubsubReconciler.ReconcileTopic(ctx, topicID, topicConfig, t.Object(), t.StatusUpdater())
	if err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.TopicID = topic.ID()

	retryPolicy := getPubsubRetryPolicy(ctx, t.DeliverySpec())
	deadLetterPolicy := getPubsubDeadLetterPolicy(projectID, t.DeliverySpec())

	// Check if PullSub exists, and if not, create it.
	subID := t.GetSubscriptionName()
	subConfig := pubsub.SubscriptionConfig{
		Topic:            topic,
		Labels:           t.GetLabels(),
		RetryPolicy:      retryPolicy,
		DeadLetterPolicy: deadLetterPolicy,
		//TODO(grantr): configure these settings?
		// AckDeadline
		// RetentionDuration
	}
	if _, err := pubsubReconciler.ReconcileSubscription(ctx, subID, subConfig, t.Object(), t.StatusUpdater()); err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.SubscriptionID = sub.ID()

	return nil
}

// getPubsubRetryPolicy gets the eventing retry policy from the Broker delivery
// spec and translates it to a pubsub retry policy.
func getPubsubRetryPolicy(ctx context.Context, spec *eventingduckv1beta1.DeliverySpec) *pubsub.RetryPolicy {
	if spec == nil {
		return &pubsub.RetryPolicy{
			MinimumBackoff: defaultMinimumBackoff,
			MaximumBackoff: defaultMaximumBackoff,
		}
	}
	// The Broker delivery spec is translated to a pubsub retry policy in the
	// manner defined in the following post:
	// https://github.com/google/knative-gcp/issues/1392#issuecomment-655617873

	var minimumBackoff time.Duration
	if spec.BackoffDelay != nil {
		p, err := period.Parse(*spec.BackoffDelay)
		if err != nil {
			// Not actually fatal, we will just use zero, rather than the stored value. But log an
			// error so that we are aware of the issue.
			logging.FromContext(ctx).Error("Unable to parse DeliverySpec.BackoffDelay",
				zap.Error(err), zap.Stringp("backoffDelay", spec.BackoffDelay))
		} else {
			minimumBackoff, _ = p.Duration()
		}
	}

	var backoffPolicy eventingduckv1beta1.BackoffPolicyType
	if spec.BackoffPolicy != nil {
		backoffPolicy = *spec.BackoffPolicy
	} else {
		// Default to Exponential.
		backoffPolicy = eventingduckv1beta1.BackoffPolicyExponential
	}

	var maximumBackoff time.Duration
	switch backoffPolicy {
	case eventingduckv1beta1.BackoffPolicyLinear:
		maximumBackoff = minimumBackoff
	case eventingduckv1beta1.BackoffPolicyExponential:
		maximumBackoff = defaultMaximumBackoff
	}
	return &pubsub.RetryPolicy{
		MinimumBackoff: minimumBackoff,
		MaximumBackoff: maximumBackoff,
	}
}

// getPubsubDeadLetterPolicy gets the eventing dead letter policy from the
// Broker delivery spec and translates it to a pubsub dead letter policy.
func getPubsubDeadLetterPolicy(projectID string, spec *eventingduckv1beta1.DeliverySpec) *pubsub.DeadLetterPolicy {
	if spec == nil || spec.DeadLetterSink == nil {
		return nil
	}
	// Translate to the pubsub dead letter policy format.

	dlp := &pubsub.DeadLetterPolicy{
		DeadLetterTopic: fmt.Sprintf("projects/%s/topics/%s", projectID, spec.DeadLetterSink.URI.Host),
	}
	if spec.Retry != nil {
		dlp.MaxDeliveryAttempts = int(*spec.Retry)
	}
	return dlp
}

func (r *TargetReconciler) DeleteRetryTopicAndSubscription(ctx context.Context, recorder record.EventRecorder, t Target) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting retry topic")

	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		t.StatusUpdater().MarkTopicUnknown("FinalizeTopicProjectIdNotFound", "Failed to find project id: %v", err)
		t.StatusUpdater().MarkSubscriptionUnknown("FinalizeSubscriptionProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}

	client, err := r.getClientOrCreateNew(ctx, projectID, t.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, recorder)

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := t.GetTopicID()
	err = multierr.Append(nil, pubsubReconciler.DeleteTopic(ctx, topicID, t.Object(), t.StatusUpdater()))
	// Delete pull subscription if it exists.
	subID := t.GetSubscriptionName()
	err = multierr.Append(err, pubsubReconciler.DeleteSubscription(ctx, subID, t.Object(), t.StatusUpdater()))
	return err
}

// getClientOrCreateNew Return the pubsubCient if it is valid, otherwise it tries to create a new client
// and register it for later usage.
func (r *TargetReconciler) getClientOrCreateNew(ctx context.Context, projectID string, b reconcilerutilspubsub.StatusUpdater) (*pubsub.Client, error) {
	if r.PubsubClient != nil {
		return r.PubsubClient, nil
	}
	client, err := CreatePubsubClientFn(ctx, projectID)
	if err != nil {
		b.MarkTopicUnknown("PubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		b.MarkSubscriptionUnknown("PubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		return nil, err
	}
	// Register the client for next run
	r.PubsubClient = client
	return client, nil
}
