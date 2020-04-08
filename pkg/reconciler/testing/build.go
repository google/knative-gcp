/*
Copyright 2019 Google LLC

Licensed under the Apache License, Veroute.on 2.0 (the "License");
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

	"knative.dev/pkg/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// CloudBuildSourceOption enables further configuration of a CloudBuildSource.
type CloudBuildSourceOption func(*v1alpha1.CloudBuildSource)

// NewCloudBuildSource creates a CloudBuildSource with CloudBuildSourceOptions
func NewCloudBuildSource(name, namespace string, so ...CloudBuildSourceOption) *v1alpha1.CloudBuildSource {
	bs := &v1alpha1.CloudBuildSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-build-uid",
		},
	}
	for _, opt := range so {
		opt(bs)
	}
	bs.SetDefaults(context.Background())
	return bs
}

func WithCloudBuildSourceSink(gvk metav1.GroupVersionKind, name string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudBuildSourceGCPServiceAccount(gServiceAccount string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Spec.GoogleServiceAccount = gServiceAccount
	}
}

func WithCloudBuildSourceDeletionTimestamp(s *v1alpha1.CloudBuildSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithCloudBuildSourceProject(project string) CloudBuildSourceOption {
	return func(s *v1alpha1.CloudBuildSource) {
		s.Spec.Project = project
	}
}

func WithCloudBuildSourceTopic(topicID string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Spec.Topic = ptr.String(topicID)
	}
}

// WithInitCloudBuildSourceConditions initializes the CloudBuildSource's conditions.
func WithInitCloudBuildSourceConditions(bs *v1alpha1.CloudBuildSource) {
	bs.Status.InitializeConditions()
}

// WithCloudBuildSourceServiceAccountName will give status.ServiceAccountName a k8s service account name, which is related on Workload Identity's Google service account.
func WithCloudBuildSourceServiceAccountName(name string) CloudBuildSourceOption {
	return func(s *v1alpha1.CloudBuildSource) {
		s.Status.ServiceAccountName = name
	}
}

func WithCloudBuildSourceWorkloadIdentityFailed(reason, message string) CloudBuildSourceOption {
	return func(s *v1alpha1.CloudBuildSource) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudBuildSourcePullSubscriptionFailed marks the condition that the
// status of PullSubscription is False
func WithCloudBuildSourcePullSubscriptionFailed(reason, message string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Status.MarkPullSubscriptionFailed(bs.ConditionSet(), reason, message)
	}
}

// WithCloudBuildSourcePullSubscriptionUnknown marks the condition that the
// topic is Unknown
func WithCloudBuildSourcePullSubscriptionUnknown(reason, message string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Status.MarkPullSubscriptionUnknown(bs.ConditionSet(), reason, message)
	}
}

// WithCloudBuildSourcePullSubscriptionReady marks the condition that the
// topic is not ready
func WithCloudBuildSourcePullSubscriptionReady() CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Status.MarkPullSubscriptionReady(bs.ConditionSet())
	}
}

// WithCloudBuildSourceSinkURI sets the status for sink URI
func WithCloudBuildSourceSinkURI(url *apis.URL) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Status.SinkURI = url
	}
}

func WithCloudBuildSourceFinalizers(finalizers ...string) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Finalizers = finalizers
	}
}

func WithCloudBuildSourceStatusObservedGeneration(generation int64) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.Status.Status.ObservedGeneration = generation
	}
}

func WithCloudBuildSourceObjectMetaGeneration(generation int64) CloudBuildSourceOption {
	return func(bs *v1alpha1.CloudBuildSource) {
		bs.ObjectMeta.Generation = generation
	}
}
