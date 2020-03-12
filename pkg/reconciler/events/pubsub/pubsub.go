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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	cloudpubsubsourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudpubsubsource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
)

const (
	resourceGroup = "cloudpubsubsources.events.cloud.google.com"

	createFailedReason      = "PullSubscriptionCreateFailed"
	getFailedReason         = "PullSubscriptionGetFailed"
	reconciledSuccessReason = "CloudPubSubSourceReconciled"
	reconciledFailedReason  = "PullSubscriptionReconcileFailed"
)

// Reconciler is the controller implementation for the CloudPubSubSource source.
type Reconciler struct {
	*reconciler.Base

	// pubsubLister for reading cloudpubsubsources.
	pubsubLister listers.CloudPubSubSourceLister
	// pullsubscriptionLister for reading pullsubscriptions.
	pullsubscriptionLister pubsublisters.PullSubscriptionLister

	receiveAdapterName string
}

// Check that our Reconciler implements Interface.
var _ cloudpubsubsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, pubsub *v1alpha1.CloudPubSubSource) pkgreconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("pubsub", pubsub)))

	pubsub.Status.InitializeConditions()
	pubsub.Status.ObservedGeneration = pubsub.Generation

	ps, event := r.reconcilePullSubscription(ctx, pubsub)
	if event != nil {
		pubsub.Status.MarkPullSubscriptionFailed(reconciledFailedReason, "Failed to reconcile PullSubscription: %s", event.Error())
		return event
	}
	pubsub.Status.PropagatePullSubscriptionStatus(&ps.Status)

	// Sink has been resolved from the underlying PullSubscription, set it here.
	sinkURI, err := apis.ParseURL(ps.Status.SinkURI)
	if err != nil {
		pubsub.Status.SinkURI = nil
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, reconciledFailedReason, "Getting sink URI failed with: %s", err.Error())
	} else {
		pubsub.Status.SinkURI = sinkURI
	}
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudPubSubSource reconciled: "%s/%s"`, pubsub.Namespace, pubsub.Name)
}

func (r *Reconciler) reconcilePullSubscription(ctx context.Context, source *v1alpha1.CloudPubSubSource) (*pubsubv1alpha1.PullSubscription, pkgreconciler.Event) {
	ps, err := r.pullsubscriptionLister.PullSubscriptions(source.Namespace).Get(source.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, getFailedReason, "Getting PullSubscription failed with: %s", err.Error())
		}
		args := &resources.PullSubscriptionArgs{
			Namespace:   source.Namespace,
			Name:        source.Name,
			Spec:        &source.Spec.PubSubSpec,
			Owner:       source,
			Topic:       source.Spec.Topic,
			Mode:        pubsubv1alpha1.ModePushCompatible,
			Labels:      resources.GetLabels(r.receiveAdapterName, source.Name),
			Annotations: resources.GetAnnotations(source.Annotations, resourceGroup),
		}
		newPS := resources.MakePullSubscription(args)
		logging.FromContext(ctx).Desugar().Debug("Creating PullSubscription", zap.Any("ps", newPS))
		ps, err = r.RunClientSet.PubsubV1alpha1().PullSubscriptions(newPS.Namespace).Create(newPS)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create PullSubscription", zap.Error(err))
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, createFailedReason, "Creating PullSubscription failed with: %s", err.Error())
		}
	}
	return ps, nil
}
