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

package v1alpha1

import (
	"github.com/google/knative-gcp/pkg/reconciler/testing"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type CloudAuditLogsSourceOption func(*v1alpha1.CloudAuditLogsSource)

func NewCloudAuditLogsSource(name, namespace string, opts ...CloudAuditLogsSourceOption) *v1alpha1.CloudAuditLogsSource {
	cal := &v1alpha1.CloudAuditLogsSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range opts {
		opt(cal)
	}
	return cal
}

func WithCloudAuditLogsSourceSink(gvk metav1.GroupVersionKind, name string) CloudAuditLogsSourceOption {
	return func(s *v1alpha1.CloudAuditLogsSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudAuditLogsSourceProject(project string) CloudAuditLogsSourceOption {
	return func(s *v1alpha1.CloudAuditLogsSource) {
		s.Spec.Project = project
	}
}

func WithCloudAuditLogsSourceResourceName(resourceName string) CloudAuditLogsSourceOption {
	return func(s *v1alpha1.CloudAuditLogsSource) {
		s.Spec.ResourceName = resourceName
	}
}

func WithCloudAuditLogsSourceServiceName(serviceName string) CloudAuditLogsSourceOption {
	return func(s *v1alpha1.CloudAuditLogsSource) {
		s.Spec.ServiceName = serviceName
	}
}

func WithCloudAuditLogsSourceServiceAccount(kServiceAccount string) CloudAuditLogsSourceOption {
	return func(s *v1alpha1.CloudAuditLogsSource) {
		s.Spec.ServiceAccountName = kServiceAccount
	}
}

func WithCloudAuditLogsSourceMethodName(methodName string) CloudAuditLogsSourceOption {
	return func(s *v1alpha1.CloudAuditLogsSource) {
		s.Spec.MethodName = methodName
	}
}
