/*
Copyright 2020 Google LLC.

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

package v1beta1

import (
	"github.com/google/knative-gcp/pkg/reconciler/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
)

// CloudPubSubSourceOption enables further configuration of a CloudPubSubSource.
type CloudPubSubSourceOption func(*v1beta1.CloudPubSubSource)

// NewCloudPubSubSource creates a CloudPubSubSource with CloudPubSubSourceOptions
func NewCloudPubSubSource(name, namespace string, so ...CloudPubSubSourceOption) *v1beta1.CloudPubSubSource {
	ps := &v1beta1.CloudPubSubSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-pubsub-uid",
		},
	}
	for _, opt := range so {
		opt(ps)
	}
	return ps
}

func WithCloudPubSubSourceSink(gvk metav1.GroupVersionKind, name string) CloudPubSubSourceOption {
	return func(ps *v1beta1.CloudPubSubSource) {
		ps.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudPubSubSourceServiceAccount(kServiceAccount string) CloudPubSubSourceOption {
	return func(ps *v1beta1.CloudPubSubSource) {
		ps.Spec.ServiceAccountName = kServiceAccount
	}
}

func WithCloudPubSubSourceTopic(topicID string) CloudPubSubSourceOption {
	return func(ps *v1beta1.CloudPubSubSource) {
		ps.Spec.Topic = topicID
	}
}
