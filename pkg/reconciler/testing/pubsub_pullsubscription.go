/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

// PubSubPullSubscriptionOption enables further configuration of a PullSubscription.
type PubSubPullSubscriptionOption func(*v1alpha1.PullSubscription)

// NewPubSubPullSubscription creates a PullSubscription with PullSubscriptionOptions
func NewPubSubPullSubscription(name, namespace string, so ...PubSubPullSubscriptionOption) *v1alpha1.PullSubscription {
	s := &v1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

// NewPubSubPullSubscriptionWithNoDefaults creates a PullSubscription with
// PullSubscriptionOptions but does not set defaults.
func NewPubSubPullSubscriptionWithNoDefaults(name, namespace string, so ...PubSubPullSubscriptionOption) *v1alpha1.PullSubscription {
	s := &v1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithPubSubPullSubscriptionUID(uid types.UID) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.UID = uid
	}
}

// WithPubSubInitPullSubscriptionConditions initializes the PullSubscriptions's conditions.
func WithPubSubInitPullSubscriptionConditions(s *v1alpha1.PullSubscription) {
	s.Status.InitializeConditions()
}

func WithPubSubPullSubscriptionSink(gvk metav1.GroupVersionKind, name string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithPubSubPullSubscriptionTransformer(gvk metav1.GroupVersionKind, name string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec.Transformer = &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithPubSubPullSubscriptionMarkSink(uri *apis.URL) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkSink(uri)
	}
}

func WithPubSubPullSubscriptionMarkTransformer(uri *apis.URL) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkTransformer(uri)
	}
}

func WithPubSubPullSubscriptionMarkNoTransformer(reason, message string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkNoTransformer(reason, message)
	}
}

func WithPubSubPullSubscriptionMarkSubscribed(subscriptionID string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkSubscribed(subscriptionID)
	}
}

func WithPubSubPullSubscriptionSubscriptionID(subscriptionID string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.SubscriptionID = subscriptionID
	}
}

func WithPubSubPullSubscriptionProjectID(projectID string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.ProjectID = projectID
	}
}

func WithPubSubPullSubscriptionTransformerURI(uri *apis.URL) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.TransformerURI = uri
	}
}

func WithPubSubPullSubscriptionMarkNoSubscription(reason, message string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkNoSubscription(reason, message)
	}
}

func WithPubSubPullSubscriptionMarkDeployed(ps *v1alpha1.PullSubscription) {
	ps.Status.MarkDeployed()
}

func WithPubSubPullSubscriptionSpec(spec v1alpha1.PullSubscriptionSpec) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec = spec
		s.Spec.SetDefaults(context.Background())
	}
}

// Same as withPullSubscriptionSpec but does not set defaults
func WithPubSubPullSubscriptionSpecWithNoDefaults(spec v1alpha1.PullSubscriptionSpec) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec = spec
	}
}

func WithPubSubPullSubscriptionReady(sink *apis.URL) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.InitializeConditions()
		s.Status.MarkSink(sink)
		s.Status.MarkDeployed()
		s.Status.MarkSubscribed("subID")
	}
}

func WithPubSubPullSubscriptionFailed() PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.InitializeConditions()
		s.Status.MarkNoSink("InvalidSink",
			`failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`)

	}
}

func WithPubSubPullSubscriptionUnknown() PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.InitializeConditions()
	}
}

func WithPubSubPullSubscriptionSinkNotFound() PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkNoSink("InvalidSink",
			`failed to get ref &ObjectReference{Kind:Sink,Namespace:testnamespace,Name:sink,UID:,APIVersion:testing.cloud.google.com/v1alpha1,ResourceVersion:,FieldPath:,}: sinks.testing.cloud.google.com "sink" not found`)
	}
}

func WithPubSubPullSubscriptionDeleted(s *v1alpha1.PullSubscription) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithPubSubPullSubscriptionOwnerReferences(ownerReferences []metav1.OwnerReference) PubSubPullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithPubSubPullSubscriptionLabels(labels map[string]string) PubSubPullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.Labels = labels
	}
}

func WithPubSubPullSubscriptionAnnotations(annotations map[string]string) PubSubPullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.Annotations = annotations
	}
}

func WithPubSubPullSubscriptionStatusObservedGeneration(generation int64) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.Status.ObservedGeneration = generation
	}
}

func WithPubSubPullSubscriptionObjectMetaGeneration(generation int64) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.ObjectMeta.Generation = generation
	}
}

func WithPubSubPullSubscriptionReadyStatus(status corev1.ConditionStatus, reason, message string) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.Conditions = []apis.Condition{{
			Type:    apis.ConditionReady,
			Status:  status,
			Reason:  reason,
			Message: message,
		}}
	}
}

func WithPubSubPullSubscriptionMode(mode v1alpha1.ModeType) PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec.Mode = mode
	}
}

func WithPubSubPullSubscriptionDeprecated() PubSubPullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkDeprecated()
	}
}
