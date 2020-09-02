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

package v1

import (
	"time"

	"github.com/google/knative-gcp/pkg/reconciler/testing"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
)

// CloudBuildSourceOption enables further configuration of a CloudBuildSource.
type CloudBuildSourceOption func(*v1.CloudBuildSource)

// NewCloudBuildSource creates a CloudBuildSource with CloudBuildSourceOptions
func NewCloudBuildSource(name, namespace string, so ...CloudBuildSourceOption) *v1.CloudBuildSource {
	bs := &v1.CloudBuildSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-build-uid",
		},
	}
	for _, opt := range so {
		opt(bs)
	}
	return bs
}

func WithCloudBuildSourceSink(gvk metav1.GroupVersionKind, name string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudBuildSourceDeletionTimestamp(bs *v1.CloudBuildSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	bs.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithCloudBuildSourceProject(project string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Spec.Project = project
	}
}

func WithCloudBuildSourceServiceAccount(kServiceAccount string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Spec.ServiceAccountName = kServiceAccount
	}
}

// WithInitCloudBuildSourceConditions initializes the CloudBuildSource's conditions.
func WithInitCloudBuildSourceConditions(bs *v1.CloudBuildSource) {
	bs.Status.InitializeConditions()
}

func WithCloudBuildSourceWorkloadIdentityFailed(reason, message string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.MarkWorkloadIdentityFailed(bs.ConditionSet(), reason, message)
	}
}

// WithCloudBuildSourcePullSubscriptionFailed marks the condition that the
// status of PullSubscription is False
func WithCloudBuildSourcePullSubscriptionFailed(reason, message string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.MarkPullSubscriptionFailed(bs.ConditionSet(), reason, message)
	}
}

// WithCloudBuildSourcePullSubscriptionUnknown marks the condition that the
// topic is Unknown
func WithCloudBuildSourcePullSubscriptionUnknown(reason, message string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.MarkPullSubscriptionUnknown(bs.ConditionSet(), reason, message)
	}
}

// WithCloudBuildSourcePullSubscriptionReady marks the condition that the
// topic is not ready
func WithCloudBuildSourcePullSubscriptionReady() CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.MarkPullSubscriptionReady(bs.ConditionSet())
	}
}

// WithCloudBuildSourceSinkURI sets the status for sink URI
func WithCloudBuildSourceSinkURI(url *apis.URL) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.SinkURI = url
	}
}

func WithCloudBuildSourceSubscriptionID(subscriptionID string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.SubscriptionID = subscriptionID
	}
}

func WithCloudBuildSourceFinalizers(finalizers ...string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Finalizers = finalizers
	}
}

func WithCloudBuildSourceStatusObservedGeneration(generation int64) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.Status.Status.ObservedGeneration = generation
	}
}

func WithCloudBuildSourceObjectMetaGeneration(generation int64) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.ObjectMeta.Generation = generation
	}
}

func WithCloudBuildSourceAnnotations(Annotations map[string]string) CloudBuildSourceOption {
	return func(bs *v1.CloudBuildSource) {
		bs.ObjectMeta.Annotations = Annotations
	}
}

func WithCloudBuildSourceSetDefault(bs *v1.CloudBuildSource) {
	bs.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
}
