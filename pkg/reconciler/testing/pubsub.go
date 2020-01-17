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
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// CloudPubSubSourceOption enables further configuration of a CloudPubSubSource.
type CloudPubSubSourceOption func(*v1alpha1.CloudPubSubSource)

// NewCloudPubSubSource creates a CloudPubSubSource with CloudPubSubSourceOptions
func NewCloudPubSubSource(name, namespace string, so ...CloudPubSubSourceOption) *v1alpha1.CloudPubSubSource {
	ps := &v1alpha1.CloudPubSubSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-CloudPubSubSource-uid",
		},
	}
	for _, opt := range so {
		opt(ps)
	}
	ps.SetDefaults(context.Background())
	return ps
}

func WithCloudPubSubSourceSink(gvk metav1.GroupVersionKind, name string) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Spec.Sink = duckv1.Destination{
			Ref: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudPubSubSourceTopic(topicID string) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Spec.Topic = topicID
	}
}

// WithInitCloudPubSubSourceConditions initializes the CloudPubSubSource's conditions.
func WithInitCloudPubSubSourceConditions(ps *v1alpha1.CloudPubSubSource) {
	ps.Status.InitializeConditions()
}

// WithCloudPubSubSourcePullSubscriptionFailed marks the condition that the
// status of PullSubscription is False
func WithCloudPubSubSourcePullSubscriptionFailed(reason, message string) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Status.MarkPullSubscriptionFailed(reason, message)
	}
}

// WithCloudPubSubSourcePullSubscriptionUnknown marks the condition that the
// topic is Unknown
func WithCloudPubSubSourcePullSubscriptionUnknown(reason, message string) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Status.MarkPullSubscriptionUnknown(reason, message)
	}
}

// WithCloudPubSubSourcePullSubscriptionReady marks the condition that the
// topic is not ready
func WithCloudPubSubSourcePullSubscriptionReady() CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Status.MarkPullSubscriptionReady()
	}
}

// WithCloudPubSubSourceSinkURI sets the status for sink URI
func WithCloudPubSubSourceSinkURI(url *apis.URL) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Status.SinkURI = url
	}
}

func WithCloudPubSubSourceFinalizers(finalizers ...string) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Finalizers = finalizers
	}
}

func WithCloudPubSubSourceStatusObservedGeneration(generation int64) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.Status.Status.ObservedGeneration = generation
	}
}

func WithCloudPubSubSourceObjectMetaGeneration(generation int64) CloudPubSubSourceOption {
	return func(ps *v1alpha1.CloudPubSubSource) {
		ps.ObjectMeta.Generation = generation
	}
}
