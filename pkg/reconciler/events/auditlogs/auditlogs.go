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

package auditlogs

import (
	"context"

	"cloud.google.com/go/logging/logadmin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	cloudauditlogssourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudauditlogssource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/events/auditlogs/resources"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
)

const (
	resourceGroup = "cloudauditlogssources.events.cloud.google.com"
	publisherRole = "roles/pubsub.publisher"

	deletePubSubFailed           = "PubSubDeleteFailed"
	deleteSinkFailed             = "SinkDeleteFailed"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	reconciledFailedReason       = "SinkReconcileFailed"
	reconciledPubSubFailedReason = "PubSubReconcileFailed"
	reconciledSuccessReason      = "CloudAuditLogsSourceReconciled"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

type Reconciler struct {
	*intevents.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	auditLogsSourceLister  listers.CloudAuditLogsSourceLister
	logadminClientProvider glogadmin.CreateFn
	pubsubClientProvider   gpubsub.CreateFn
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister
}

// Check that our Reconciler implements Interface.
var _ cloudauditlogssourcereconciler.Interface = (*Reconciler)(nil)

func (c *Reconciler) ReconcileKind(ctx context.Context, s *v1.CloudAuditLogsSource) reconciler.Event {
	ctx = logging.WithLogger(ctx, c.Logger.With(zap.Any("auditlogsource", s)))

	s.Status.InitializeConditions()
	s.Status.ObservedGeneration = s.Generation

	// If ServiceAccountName is provided, reconcile workload identity.
	if s.Spec.ServiceAccountName != "" {
		if _, err := c.Identity.ReconcileWorkloadIdentity(ctx, s.Spec.Project, s); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile CloudAuditLogsSource workload identity: %s", err.Error())
		}
	}

	topic := resources.GenerateTopicName(s)
	t, ps, err := c.PubSubBase.ReconcilePubSub(ctx, s, topic, resourceGroup)
	if err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: %s", err.Error())
	}
	c.Logger.Debugf("Reconciled: PubSub: %+v PullSubscription: %+v", t, ps)

	sink, err := c.reconcileSink(ctx, s)
	if err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Sink failed with: %s", err.Error())
	}
	s.Status.StackdriverSink = sink
	s.Status.MarkSinkReady()
	c.Logger.Debugf("Reconciled Stackdriver sink: %+v", sink)

	return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudAuditLogsSource reconciled: "%s/%s"`, s.Namespace, s.Name)
}

func (c *Reconciler) reconcileSink(ctx context.Context, s *v1.CloudAuditLogsSource) (string, error) {
	sink, err := c.ensureSinkCreated(ctx, s)
	if err != nil {
		s.Status.MarkSinkNotReady("SinkCreateFailed", "failed to ensure creation of logging sink: %s", err.Error())
		return "", err
	}
	err = c.ensureSinkIsPublisher(ctx, s, sink)
	if err != nil {
		s.Status.MarkSinkNotReady("SinkNotPublisher", "failed to ensure sink has pubsub.publisher permission on source topic: %s", err.Error())
		return "", err
	}
	return sink.ID, nil
}

func (c *Reconciler) ensureSinkCreated(ctx context.Context, s *v1.CloudAuditLogsSource) (*logadmin.Sink, error) {
	sinkID := s.Status.StackdriverSink
	if sinkID == "" {
		sinkID = resources.GenerateSinkName(s)
	}
	logadminClient, err := c.logadminClientProvider(ctx, s.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create LogAdmin client", zap.Error(err))
		return nil, err
	}
	sink, err := logadminClient.Sink(ctx, sinkID)
	if status.Code(err) == codes.NotFound {
		filterBuilder := resources.FilterBuilder{}
		filterBuilder.WithServiceName(s.Spec.ServiceName).WithMethodName(s.Spec.MethodName)
		if s.Spec.ResourceName != "" {
			filterBuilder.WithResourceName(s.Spec.ResourceName)
		}
		sink = &logadmin.Sink{
			ID:          sinkID,
			Destination: resources.GenerateTopicResourceName(s),
			Filter:      filterBuilder.GetFilterQuery(),
		}
		sink, err = logadminClient.CreateSinkOpt(ctx, sink, logadmin.SinkOptions{UniqueWriterIdentity: true})
		// Handle AlreadyExists in-case of a race between another create call.
		if status.Code(err) == codes.AlreadyExists {
			sink, err = logadminClient.Sink(ctx, sinkID)
		}
	}
	return sink, err
}

// Ensures that the sink has been granted the pubsub.publisher role on the source topic.
func (c *Reconciler) ensureSinkIsPublisher(ctx context.Context, s *v1.CloudAuditLogsSource, sink *logadmin.Sink) error {
	pubsubClient, err := c.pubsubClientProvider(ctx, s.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create PubSub client", zap.Error(err))
		return err
	}
	topicIam := pubsubClient.Topic(s.Status.TopicID).IAM()
	topicPolicy, err := topicIam.Policy(ctx)
	if err != nil {
		return err
	}
	if !topicPolicy.HasRole(sink.WriterIdentity, publisherRole) {
		topicPolicy.Add(sink.WriterIdentity, publisherRole)
		if err = topicIam.SetPolicy(ctx, topicPolicy); err != nil {
			return err
		}
		logging.FromContext(ctx).Desugar().Debug(
			"Granted the Stackdriver Sink writer identity roles/pubsub.publisher on PubSub Topic.",
			zap.String("writerIdentity", sink.WriterIdentity),
			zap.String("topicID", s.Status.TopicID))
	}
	return nil
}

// deleteSink looks at status.SinkID and if non-empty will delete the
// previously created stackdriver sink.
func (c *Reconciler) deleteSink(ctx context.Context, s *v1.CloudAuditLogsSource) error {
	if s.Status.StackdriverSink == "" {
		return nil
	}
	logadminClient, err := c.logadminClientProvider(ctx, s.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create LogAdmin client", zap.Error(err))
		s.Status.MarkSinkUnknown(deleteSinkFailed, "Failed to create LogAdmin Client: %s", err.Error())
		return err
	}
	if err = logadminClient.DeleteSink(ctx, s.Status.StackdriverSink); err != nil && status.Code(err) != codes.NotFound {
		s.Status.MarkSinkUnknown(deleteSinkFailed, "Failed to delete Stackdriver sink: %s", err.Error())
		return err
	}
	return nil
}

func (c *Reconciler) FinalizeKind(ctx context.Context, s *v1.CloudAuditLogsSource) reconciler.Event {
	// If k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if s.Spec.ServiceAccountName != "" {
		if err := c.Identity.DeleteWorkloadIdentity(ctx, s.Spec.Project, s); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete CloudAuditLogsSource workload identity: %s", err.Error())
		}
	}

	if err := c.deleteSink(ctx, s); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deleteSinkFailed, "Failed to delete Stackdriver sink: %s", err.Error())
	}

	if err := c.PubSubBase.DeletePubSub(ctx, s); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deletePubSubFailed, "Failed to delete CloudAuditLogsSource PubSub: %s", err.Error())
	}
	s.Status.StackdriverSink = ""
	return nil
}
