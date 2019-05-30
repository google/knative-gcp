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

package pubsubsource

import (
	"cloud.google.com/go/pubsub"
	"context"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
	listers "github.com/GoogleCloudPlatform/cloud-run-events/pkg/client/listers/eventing/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsubsource/resources"
	"github.com/knative/pkg/controller"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
	//"github.com/GoogleCloudPlatform/cloud-run-events/pkg/controller/sdk"
	//"github.com/GoogleCloudPlatform/cloud-run-events/pkg/controller/sinks"
	//"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/eventtype"
	//"github.com/GoogleCloudPlatform/cloud-run-events/pkg/reconciler/pubsubsource/resources"
	//eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/apimachinery/pkg/labels"
	//"k8s.io/apimachinery/pkg/runtime"
	//"k8s.io/apimachinery/pkg/runtime/schema"
	//"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "PubSubSources"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "GCPPUBSUB_RA_IMAGE"

	finalizerName = controllerAgentName
)

// Reconciler implements controller.Reconciler for PubSubSource resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	sourceLister listers.PubSubSourceLister

	receiveAdapterImage string
	//	eventTypeReconciler eventtype.Reconciler
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

//
//// Add creates a new PubSub Controller and adds it to the Manager with
//// default RBAC. The Manager will set fields on the Controller and Start it when
//// the Manager is Started.
//func Add(mgr manager.Manager, logger *zap.SugaredLogger) error {
//	raImage, defined := os.LookupEnv(raImageEnvVar)
//	if !defined {
//		return fmt.Errorf("required environment variable '%s' not defined", raImageEnvVar)
//	}
//
//	log.Println("Adding the GCP PubSub Source controller.")
//	p := &sdk.Provider{
//		AgentName: controllerAgentName,
//		Parent:    &v1alpha1.PubSub{},
//		Owns:      []runtime.Object{&v1.Deployment{}, &eventingv1alpha1.EventType{}},
//		Reconciler: &reconciler{
//			scheme:              mgr.GetScheme(),
//			pubSubClientCreator: gcpPubSubClientCreator,
//			receiveAdapterImage: raImage,
//			eventTypeReconciler: eventtype.Reconciler{
//				Scheme: mgr.GetScheme(),
//			},
//		},
//	}
//
//	return p.Add(mgr, logger)
//}

// gcpPubSubClientCreator creates a real GCP PubSub client. It should always be used, except during
// unit tests.
func gcpPubSubClientCreator(ctx context.Context, googleCloudProject string) (pubSubClient, error) {
	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	psc, err := pubsub.NewClient(ctx, googleCloudProject)
	if err != nil {
		return nil, err
	}
	return &realGcpPubSubClient{
		client: psc,
	}, nil
}

//type Reconciler struct {
//	client client.Client
//	scheme *runtime.Scheme
//
//	pubSubClientCreator pkg.pubSubClientCreator
//	receiveAdapterImage string
//	eventTypeReconciler eventtype.Reconciler
//}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Service resource
// with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	logger := logging.FromContext(ctx)

	// Get the Service resource with this namespace/name
	original, err := c.sourceLister.PubSubSources(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("service %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	if original.GetDeletionTimestamp() != nil {
		return nil
	}

	// Don't modify the informers copy
	source := original.DeepCopy()

	// Reconcile this copy of the source and then write back any status
	// updates regardless of whether the reconciliation errored out.
	var reconcileErr = c.reconcile(ctx, source)

	if equality.Semantic.DeepEqual(original.Status, source.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := c.updateStatus(source); uErr != nil {
		logger.Warnw("Failed to update source status", zap.Error(uErr))
		c.Recorder.Eventf(source, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for PubSubSource %q: %v", source.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		c.Recorder.Eventf(source, corev1.EventTypeNormal, "Updated", "Updated PubSubSource %q", source.GetName())
	}
	if reconcileErr != nil {
		c.Recorder.Event(source, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (c *Reconciler) reconcile(ctx context.Context, source *v1alpha1.PubSubSource) error {
	logger := logging.FromContext(ctx)

	source.Status.InitializeConditions()

	// TODO: Rec this source.
	logger.Info()
	_ = logger

	source.Status.ObservedGeneration = source.Generation

	return nil
}

func (c *Reconciler) updateStatus(desired *v1alpha1.PubSubSource) (*v1alpha1.PubSubSource, error) {
	source, err := c.sourceLister.PubSubSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(source.Status, desired.Status) {
		return source, nil
	}
	becomesReady := desired.Status.IsReady() && !source.Status.IsReady()
	// Don't modify the informers copy.
	existing := source.DeepCopy()
	existing.Status = desired.Status

	src, err := c.RunClientSet.EventsV1alpha1().PubSubSources(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("PubSubSource %q became ready after %v", source.Name, duration)
		// c.StatsReporter.ReportServiceReady(service.Namespace, service.Name, duration) // TODO: Stats
	}

	return src, err
}

//
//func (r *Reconciler) Reconcile_old(ctx context.Context, object runtime.Object) error {
//	logger := logging.FromContext(ctx).Desugar()
//
//	src, ok := object.(*v1alpha1.PubSubSource)
//	if !ok {
//		logger.Error("could not find GcpPubSub source", zap.Any("object", object))
//		return nil
//	}
//
//	// This Source attempts to reconcile three things.
//	// 1. Determine the sink's URI.
//	//     - Nothing to delete.
//	// 2. Create a receive adapter in the form of a Deployment.
//	//     - Will be garbage collected by K8s when this PubSub is deleted.
//	// 3. Register that receive adapter as a Pull endpoint for the specified GCP PubSub Topic.
//	//     - This needs to deregister during deletion.
//	// 4. Create the EventTypes that it can emit.
//	//     - Will be garbage collected by K8s when this PubSub is deleted.
//	// Because there is something that must happen during deletion, we add this controller as a
//	// finalizer to every PubSub.
//
//	// See if the source has been deleted.
//	deletionTimestamp := src.DeletionTimestamp
//	if deletionTimestamp != nil {
//		err := r.deleteSubscription(ctx, src)
//		if err != nil {
//			logger.Error("Unable to delete the Subscription", zap.Error(err))
//			return err
//		}
//		r.removeFinalizer(src)
//		return nil
//	}
//
//	r.addFinalizer(src)
//
//	src.Status.InitializeConditions()
//
//	sinkURI, err := sinks.GetSinkURI(ctx, r.client, src.Spec.Sink, src.Namespace)
//	if err != nil {
//		src.Status.MarkNoSink("NotFound", "")
//		return err
//	}
//	src.Status.MarkSink(sinkURI)
//
//	sub, err := r.createSubscription(ctx, src)
//	if err != nil {
//		logger.Error("Unable to create the subscription", zap.Error(err))
//		return err
//	}
//	src.Status.MarkSubscribed()
//
//	_, err = r.createReceiveAdapter(ctx, src, sub.ID(), sinkURI)
//	if err != nil {
//		logger.Error("Unable to create the receive adapter", zap.Error(err))
//		return err
//	}
//	src.Status.MarkDeployed()
//
//	// Only create EventTypes for Broker sinks.
//	if src.Spec.Sink.Kind == "Broker" {
//		err = r.reconcileEventTypes(ctx, src)
//		if err != nil {
//			logger.Error("Unable to reconcile the event types", zap.Error(err))
//			return err
//		}
//		src.Status.MarkEventTypes()
//	}
//
//	return nil
//}
//
//func (r *Reconciler) addFinalizer(s *v1alpha1.PubSub) {
//	finalizers := sets.NewString(s.Finalizers...)
//	finalizers.Insert(finalizerName)
//	s.Finalizers = finalizers.List()
//}
//
//func (r *Reconciler) removeFinalizer(s *v1alpha1.PubSub) {
//	finalizers := sets.NewString(s.Finalizers...)
//	finalizers.Delete(finalizerName)
//	s.Finalizers = finalizers.List()
//}
//
func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.PubSubSource, subscriptionID, sinkURI string) (*appsv1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		return ra, nil
	}
	dp := resources.MakeReceiveAdapter(&resources.ReceiveAdapterArgs{
		Image:          r.receiveAdapterImage,
		Source:         src,
		Labels:         resources.GetLabels(controllerAgentName, src.Name),
		SubscriptionID: subscriptionID,
		SinkURI:        sinkURI,
	})
	//if err := controllerutil.SetControllerReference(src, svc, r.scheme); err != nil { // TODO: use kmeta
	//	return nil, err
	//}
	dp, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(dp)
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Error(err), zap.Any("receiveAdapter", dp))
	return dp, err
}

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.PubSubSource) (*appsv1.Deployment, error) {

	dl, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: resources.GetLabelSelector(controllerAgentName, src.Name).String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

//func (r *Reconciler) createSubscription(ctx context.Context, src *v1alpha1.PubSub) (pkg.pubSubSubscription, error) {
//	psc, err := r.pubSubClientCreator(ctx, src.Spec.GoogleCloudProject)
//	if err != nil {
//		return nil, err
//	}
//	sub := psc.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject)
//	if exists, err := sub.Exists(ctx); err != nil {
//		return nil, err
//	} else if exists {
//		logging.FromContext(ctx).Info("Reusing existing subscription.")
//		return sub, nil
//	}
//	createdSub, err := psc.CreateSubscription(ctx, sub.ID(), pubsub.SubscriptionConfig{
//		Topic: psc.Topic(src.Spec.Topic),
//	})
//	if err != nil {
//		logging.FromContext(ctx).Desugar().Info("Error creating new subscription", zap.Error(err))
//	} else {
//		logging.FromContext(ctx).Desugar().Info("Created new subscription", zap.Any("subscription", createdSub))
//	}
//	return createdSub, err
//}
//
//func (r *Reconciler) deleteSubscription(ctx context.Context, src *v1alpha1.PubSub) error {
//	psc, err := r.pubSubClientCreator(ctx, src.Spec.GoogleCloudProject)
//	if err != nil {
//		return err
//	}
//	sub := psc.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject)
//	if exists, err := sub.Exists(ctx); err != nil {
//		return err
//	} else if !exists {
//		return nil
//	}
//	return sub.Delete(ctx)
//}
//
//func (r *Reconciler) reconcileEventTypes(ctx context.Context, src *v1alpha1.PubSubSource) error {
//	args := r.newEventTypeReconcilerArgs(src)
//	return r.eventTypeReconciler.Reconcile(ctx, src, args)
//}
//
//func (r *Reconciler) newEventTypeReconcilerArgs(src *v1alpha1.PubSub) *eventtype.ReconcilerArgs {
//	spec := eventingv1alpha1.EventTypeSpec{
//		Type:   v1alpha1.PubSubEventType,
//		Source: v1alpha1.GetPubSub(src.Spec.GoogleCloudProject, src.Spec.Topic),
//		Broker: src.Spec.Sink.Name,
//	}
//	specs := make([]eventingv1alpha1.EventTypeSpec, 0, 1)
//	specs = append(specs, spec)
//	return &eventtype.ReconcilerArgs{
//		Specs:     specs,
//		Namespace: src.Namespace,
//		Labels:    getLabels(src),
//	}
//}
//
//func generateSubName(src *v1alpha1.PubSubSource) string {
//	return fmt.Sprintf("knative-eventing-%s-%s-%s", src.Namespace, src.Name, src.UID)
//}
