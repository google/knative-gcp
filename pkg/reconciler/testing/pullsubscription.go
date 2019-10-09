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
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

// PullSubscriptionOption enables further configuration of a PullSubscription.
type PullSubscriptionOption func(*v1alpha1.PullSubscription)

// NewPullSubscription creates a PullSubscription with PullSubscriptionOptions
func NewPullSubscription(name, namespace string, so ...PullSubscriptionOption) *v1alpha1.PullSubscription {
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

// NewPullSubscriptionWithNoDefaults creates a PullSubscription with
// PullSubscriptionOptions but does not set defaults.
func NewPullSubscriptionWithNoDefaults(name, namespace string, so ...PullSubscriptionOption) *v1alpha1.PullSubscription {
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

// NewPullSubscriptionWithoutNamespace creates a PullSubscription with PullSubscriptionOptions but without a specific namespace
func NewPullSubscriptionWithoutNamespace(name string, so ...PullSubscriptionOption) *v1alpha1.PullSubscription {
	s := &v1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithPullSubscriptionUID(uid types.UID) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.UID = uid
	}
}

func WithPullSubscriptionGenerateName(generateName string) PullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.GenerateName = generateName
	}
}

// WithInitPullSubscriptionConditions initializes the PullSubscriptions's conditions.
func WithInitPullSubscriptionConditions(s *v1alpha1.PullSubscription) {
	s.Status.InitializeConditions()
}

func WithPullSubscriptionSink(gvk metav1.GroupVersionKind, name string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec.Sink = apisv1alpha1.Destination{
			ObjectReference: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithPullSubscriptionMarkSink(uri string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkSink(uri)
	}
}

func WithPullSubscriptionSubscription(subscriptionID string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkSubscribed()
		s.Status.SubscriptionID = subscriptionID
	}
}

func WithPullSubscriptionMarkSubscribing(subscriptionID string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkSubscriptionOperation("Creating", "Created Job to create Subscription %q.", subscriptionID)
		s.Status.SubscriptionID = subscriptionID
	}
}

func WithPullSubscriptionMarkUnsubscribing(subscriptionID string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkSubscriptionOperation("Deleting", "Created Job to delete Subscription %q.", subscriptionID)
	}
}

func WithPullSubscriptionMarkNoSubscription(subscriptionID string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkNoSubscription("Deleted", "Successfully deleted Subscription %q.", subscriptionID)
		s.Status.SubscriptionID = ""
	}
}

func WithPullSubscriptionSpec(spec v1alpha1.PullSubscriptionSpec) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec = spec
		s.Spec.SetDefaults(context.Background())
	}
}

// Same as withPullSubscriptionSpec but does not set defaults
func WithPullSubscriptionSpecWithNoDefaults(spec v1alpha1.PullSubscriptionSpec) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Spec = spec
	}
}

func WithPullSubscriptionReady(sink string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.InitializeConditions()
		s.Status.MarkSink(sink)
		s.Status.MarkDeployed()
		s.Status.MarkSubscribed()
	}
}

//func WithPullSubscriptionProjectResolved(projectID string) PullSubscriptionOption {
//	return func(s *v1alpha1.PullSubscription) {
//		s.Status.ProjectID = projectID
//	}
//}

func WithPullSubscriptionSinkNotFound() PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.MarkNoSink("InvalidSink", `sinks.testing.cloud.google.com "sink" not found`)
	}
}

func WithPullSubscriptionDeleted(s *v1alpha1.PullSubscription) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithPullSubscriptionOwnerReferences(ownerReferences []metav1.OwnerReference) PullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithPullSubscriptionLabels(labels map[string]string) PullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.Labels = labels
	}
}

func WithPullSubscriptionAnnotations(annotations map[string]string) PullSubscriptionOption {
	return func(c *v1alpha1.PullSubscription) {
		c.ObjectMeta.Annotations = annotations
	}
}

func WithPullSubscriptionFinalizers(finalizers ...string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Finalizers = finalizers
	}
}

func WithPullSubscriptionStatusObservedGeneration(generation int64) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.Status.ObservedGeneration = generation
	}
}

func WithPullSubscriptionObjectMetaGeneration(generation int64) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.ObjectMeta.Generation = generation
	}
}

func WithPullSubscriptionReadyStatus(status corev1.ConditionStatus, reason, message string) PullSubscriptionOption {
	return func(s *v1alpha1.PullSubscription) {
		s.Status.Conditions = []apis.Condition{{
			Type:    apis.ConditionReady,
			Status:  status,
			Reason:  reason,
			Message: message,
		}}
	}
}
