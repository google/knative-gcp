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

package topic

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/tracing"
	"github.com/google/knative-gcp/pkg/utils"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	tracingconfig "knative.dev/pkg/tracing/config"

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	gstatus "google.golang.org/grpc/status"

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	topicreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1/topic"
	listers "github.com/google/knative-gcp/pkg/client/listers/intevents/v1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/topic/resources"
)

const (
	resourceGroup = "topics.internal.events.cloud.google.com"

	deleteTopicFailed               = "TopicDeleteFailed"
	deleteWorkloadIdentityFailed    = "WorkloadIdentityDeleteFailed"
	reconciledPublisherFailedReason = "PublisherReconcileFailed"
	reconciledSuccessReason         = "TopicReconciled"
	reconciledTopicFailedReason     = "TopicReconcileFailed"
	workloadIdentityFailed          = "WorkloadIdentityReconcileFailed"
)

// Reconciler implements controller.Reconciler for Topic resources.
type Reconciler struct {
	*intevents.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// data residency store
	dataresidencyStore *dataresidency.Store
	// topicLister index properties about topics.
	topicLister listers.TopicLister
	// serviceLister index properties about services.
	serviceLister servinglisters.ServiceLister
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister

	publisherImage string
	tracingConfig  *tracingconfig.Config

	// createClientFn is the function used to create the Pub/Sub client that interacts with Pub/Sub.
	// This is needed so that we can inject a mock client for UTs purposes.
	createClientFn gpubsub.CreateFn
}

// Check that our Reconciler implements Interface.
var _ topicreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, topic *v1.Topic) reconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("topic", topic)))

	topic.Status.InitializeConditions()
	topic.Status.ObservedGeneration = topic.Generation

	// If topic doesn't have ownerReference and ServiceAccountName is provided, reconcile workload identity.
	// Otherwise, its owner will reconcile workload identity.
	if (topic.OwnerReferences == nil || len(topic.OwnerReferences) == 0) && topic.Spec.ServiceAccountName != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, topic.Spec.Project, topic); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile Pub/Sub topic workload identity: %s", err.Error())
		}
	}

	if err := r.reconcileTopic(ctx, topic); err != nil {
		topic.Status.MarkNoTopic(reconciledTopicFailedReason, "Failed to reconcile Pub/Sub topic: %s", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledTopicFailedReason, "Failed to reconcile Pub/Sub topic: %s", err.Error())
	}
	topic.Status.MarkTopicReady()
	// Set the topic being used.
	topic.Status.TopicID = topic.Spec.Topic

	// If enablePublisher is false, then skip creating the publisher.
	if enablePublisher := topic.Spec.EnablePublisher; enablePublisher != nil && !*enablePublisher {
		return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, topic.Namespace, topic.Name)
	}

	err, svc := r.reconcilePublisher(ctx, topic)
	if err != nil {
		topic.Status.MarkPublisherNotDeployed(reconciledPublisherFailedReason, "Failed to reconcile Publisher: %s", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledPublisherFailedReason, "Failed to reconcile Publisher: %s", err.Error())
	}

	// Update the topic.
	topic.Status.PropagatePublisherStatus(&svc.Status)

	return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `Topic reconciled: "%s/%s"`, topic.Namespace, topic.Name)
}

func (r *Reconciler) reconcileTopic(ctx context.Context, topic *v1.Topic) error {
	if topic.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(topic.Spec.Project, metadataClient.NewDefaultMetadataClient())
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return err
		}
		// Set the projectID in the status.
		topic.Status.ProjectID = projectID
	}

	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	client, err := r.createClientFn(ctx, topic.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	t := client.Topic(topic.Spec.Topic)
	exists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return err
	}

	if !exists {
		if topic.Spec.PropagationPolicy == v1.TopicPolicyNoCreateNoDelete {
			logging.FromContext(ctx).Desugar().Error("Topic does not exist and the topic policy doesn't allow creation")
			return fmt.Errorf("Topic %q does not exist and the topic policy doesn't allow creation", topic.Spec.Topic)
		} else {
			topicConfig := &pubsub.TopicConfig{}
			if r.dataresidencyStore != nil {
				if dataresidencyCfg := r.dataresidencyStore.Load(); dataresidencyCfg != nil {
					if dataresidencyCfg.DataResidencyDefaults.ComputeAllowedPersistenceRegions(topicConfig) {
						r.Logger.Debugw("Updated Topic Config AllowedPersistenceRegions for topic reconciler", zap.Any("topicConfig", *topicConfig))
					}
				}
			}
			// Create a new topic with the given name.
			t, err = client.CreateTopicWithConfig(ctx, topic.Spec.Topic, topicConfig)
			if err != nil {
				// For some reason (maybe some cache invalidation thing), sometimes t.Exists returns that the topic
				// doesn't exist but it actually does. When we try to create it again, it fails with an AlreadyExists
				// reason. We check for that error here. If it happens, then return nil.
				if st, ok := gstatus.FromError(err); !ok {
					logging.FromContext(ctx).Desugar().Error("Failed from Pub/Sub client while creating topic", zap.Error(err))
					return err
				} else if st.Code() != codes.AlreadyExists {
					logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub topic", zap.Error(err))
					return err
				}
				return nil
			}
		}
	}
	return nil
}

// deleteTopic looks at the status.TopicID and if non-empty,
// hence indicating that we have created a topic successfully,
// remove it.
func (r *Reconciler) deleteTopic(ctx context.Context, topic *v1.Topic) error {
	if topic.Status.TopicID == "" {
		return nil
	}

	// At this point the project ID should have been populated in the status.
	// Querying Pub/Sub as the topic could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.createClientFn(ctx, topic.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	t := client.Topic(topic.Status.TopicID)
	exists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return err
	}
	if exists {
		// Delete the topic.
		if err := t.Delete(ctx); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to delete Pub/Sub topic", zap.Error(err))
			return err
		}
	}
	return nil
}

func (r *Reconciler) reconcilePublisher(ctx context.Context, topic *v1.Topic) (error, *servingv1.Service) {
	name := resources.GeneratePublisherName(topic)
	existing, err := r.serviceLister.Services(topic.Namespace).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Unable to get an existing publisher", zap.Error(err))
			return err, nil
		}
		existing = nil
	} else if !metav1.IsControlledBy(existing, topic) {
		p, _ := json.Marshal(existing)
		logging.FromContext(ctx).Desugar().Error("Topic does not own publisher service", zap.Any("publisher", p))
		return fmt.Errorf("Topic %q does not own publisher service: %q", topic.Name, name), nil
	}

	tracingCfg, err := tracing.ConfigToJSON(r.tracingConfig)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Error serializing tracing config", zap.Error(err))
	}

	desired := resources.MakePublisher(&resources.PublisherArgs{
		Image:         r.publisherImage,
		Topic:         topic,
		Labels:        resources.GetLabels(controllerAgentName, topic.Name),
		TracingConfig: tracingCfg,
	})

	svc := existing
	if existing == nil {
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Create(ctx, desired, metav1.CreateOptions{})
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create publisher", zap.Error(err))
			return err, nil
		}
	} else if !equality.Semantic.DeepEqual(&existing.Spec, &desired.Spec) {
		existing.Spec = desired.Spec
		svc, err = r.ServingClientSet.ServingV1().Services(topic.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update publisher", zap.Any("publisher", existing), zap.Error(err))
			return err, nil
		}
	}
	return nil, svc
}

func (r *Reconciler) UpdateFromTracingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		r.Logger.Error("Tracing ConfigMap is nil")
		return
	}
	delete(cfg.Data, "_example")

	tracingCfg, err := tracingconfig.NewTracingConfigFromConfigMap(cfg)
	if err != nil {
		r.Logger.Warnw("failed to create tracing config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.tracingConfig = tracingCfg
	r.Logger.Debugw("Updated Tracing config", zap.Any("tracingCfg", r.tracingConfig))
	// TODO: requeue all Topics. See https://github.com/google/knative-gcp/issues/457.
}

func (r *Reconciler) FinalizeKind(ctx context.Context, topic *v1.Topic) reconciler.Event {
	// If topic doesn't have ownerReference, and
	// k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if (topic.OwnerReferences == nil || len(topic.OwnerReferences) == 0) && topic.Spec.ServiceAccountName != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, topic.Spec.Project, topic); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete delete Pub/Sub topic workload identity: %s", err.Error())
		}
	}
	if topic.Spec.PropagationPolicy == v1.TopicPolicyCreateDelete {
		logging.FromContext(ctx).Desugar().Debug("Deleting Pub/Sub topic")
		if err := r.deleteTopic(ctx, topic); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteTopicFailed, "Failed to delete Pub/Sub topic: %s", err.Error())
		}
	}
	return nil
}
