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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// PubSubOption enables further configuration of a PubSub.
type PubSubOption func(*v1alpha1.PubSub)

// NewPubSub creates a PubSub with PubSubOptions
func NewPubSub(name, namespace string, so ...PubSubOption) *v1alpha1.PubSub {
	ps := &v1alpha1.PubSub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-pubsub-uid",
		},
	}
	for _, opt := range so {
		opt(ps)
	}
	ps.SetDefaults(context.Background())
	return ps
}

func WithPubSubSink(gvk metav1.GroupVersionKind, name string) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Spec.Sink = apisv1alpha1.Destination{
			Ref: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithPubSubTopic(topicID string) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Spec.Topic = topicID
	}
}

// WithInitPubSubConditions initializes the PubSub's conditions.
func WithInitPubSubConditions(ps *v1alpha1.PubSub) {
	ps.Status.InitializeConditions()
}

// WithPubSubPullSubscriptionNotReady marks the condition that the
// topic is not ready
func WithPubSubPullSubscriptionNotReady(reason, message string) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Status.MarkPullSubscriptionNotReady(reason, message)
	}
}

// WithPubSubPullSubscriptionReady marks the condition that the
// topic is not ready
func WithPubSubPullSubscriptionReady() PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Status.MarkPullSubscriptionReady()
	}
}

// WithPubSubSinkURI sets the status for sink URI
func WithPubSubSinkURI(url *apis.URL) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Status.SinkURI = url
	}
}

func WithPubSubFinalizers(finalizers ...string) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Finalizers = finalizers
	}
}

func WithPubSubStatusObservedGeneration(generation int64) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.Status.Status.ObservedGeneration = generation
	}
}

func WithPubSubObjectMetaGeneration(generation int64) PubSubOption {
	return func(ps *v1alpha1.PubSub) {
		ps.ObjectMeta.Generation = generation
	}
}
