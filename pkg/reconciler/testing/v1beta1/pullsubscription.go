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

	"github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
)

// PullSubscriptionOption enables further configuration of a PullSubscription.
type PullSubscriptionOption func(*v1beta1.PullSubscription)

const (
	SubscriptionID = "subID"
)

// NewPullSubscription creates a PullSubscription with PullSubscriptionOptions
func NewPullSubscription(name, namespace string, so ...PullSubscriptionOption) *v1beta1.PullSubscription {
	s := &v1beta1.PullSubscription{
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

func WithPullSubscriptionSink(gvk metav1.GroupVersionKind, name string) PullSubscriptionOption {
	return func(s *v1beta1.PullSubscription) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithPullSubscriptionSpec(spec v1beta1.PullSubscriptionSpec) PullSubscriptionOption {
	return func(s *v1beta1.PullSubscription) {
		s.Spec = spec
	}
}

func WithPullSubscriptionTopic(topicID string) PullSubscriptionOption {
	return func(s *v1beta1.PullSubscription) {
		s.Spec.Topic = topicID
	}
}

func WithPullSubscriptionServiceAccount(kServiceAccount string) PullSubscriptionOption {
	return func(s *v1beta1.PullSubscription) {
		s.Spec.ServiceAccountName = kServiceAccount
	}
}
