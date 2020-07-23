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

// CloudSchedulerSourceOption enables further configuration of a CloudSchedulerSource.
type CloudSchedulerSourceOption func(*v1beta1.CloudSchedulerSource)

// NewCloudSchedulerSource creates a CloudSchedulerSource with CloudSchedulerSourceOptions
func NewCloudSchedulerSource(name, namespace string, so ...CloudSchedulerSourceOption) *v1beta1.CloudSchedulerSource {
	s := &v1beta1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-scheduler-uid",
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithCloudSchedulerSourceSink(gvk metav1.GroupVersionKind, name string) CloudSchedulerSourceOption {
	return func(s *v1beta1.CloudSchedulerSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudSchedulerSourceLocation(location string) CloudSchedulerSourceOption {
	return func(s *v1beta1.CloudSchedulerSource) {
		s.Spec.Location = location
	}
}

func WithCloudSchedulerSourceSchedule(schedule string) CloudSchedulerSourceOption {
	return func(s *v1beta1.CloudSchedulerSource) {
		s.Spec.Schedule = schedule
	}
}

func WithCloudSchedulerSourceServiceAccount(kServiceAccount string) CloudSchedulerSourceOption {
	return func(ps *v1beta1.CloudSchedulerSource) {
		ps.Spec.ServiceAccountName = kServiceAccount
	}
}

func WithCloudSchedulerSourceData(data string) CloudSchedulerSourceOption {
	return func(s *v1beta1.CloudSchedulerSource) {
		s.Spec.Data = data
	}
}
