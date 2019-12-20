/*
Copyright 2019 Google LLC.

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

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type CloudAuditLogOption func(*v1alpha1.CloudAuditLog)

func NewCloudAuditLog(name, namespace string, opts ...CloudAuditLogOption) *v1alpha1.CloudAuditLog {
	cal := &v1alpha1.CloudAuditLog{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-cloudauditlog-uid",
		},
	}
	for _, opt := range opts {
		opt(cal)
	}
	cal.SetDefaults(context.Background())
	return cal
}

func WithInitCloudAuditLogConditions(cal *v1alpha1.CloudAuditLog) {
	cal.Status.InitializeConditions()
}

func WithCloudAuditLogTopicNotReady(reason, message string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.MarkTopicNotReady(reason, message)
	}
}

func WithCloudAuditLogTopicReady(topicID string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.MarkTopicReady()
		cal.Status.TopicID = topicID
	}
}

func WithCloudAuditLogPullSubscriptionNotReady(reason, message string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.MarkPullSubscriptionNotReady(reason, message)
	}
}

func WithCloudAuditLogPullSubscriptionReady() CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.MarkPullSubscriptionReady()
	}
}

func WithCloudAuditLogSinkNotReady(reason, messageFmt string, messageA ...interface{}) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.MarkSinkNotReady(reason, messageFmt, messageA...)
	}
}

func WithCloudAuditLogSinkReady() CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.MarkSinkReady()
	}
}

func WithCloudAuditLogSink(gvk metav1.GroupVersionKind, name string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Spec.Sink = duckv1.Destination{
			Ref: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudAuditLogSinkURI(url *apis.URL) CloudAuditLogOption {
	return func(s *v1alpha1.CloudAuditLog) {
		s.Status.SinkURI = url
	}
}

func WithCloudAuditLogProjectID(projectID string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.ProjectID = projectID
	}
}

func WithCloudAuditLogSinkID(sinkID string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Status.SinkID = sinkID
	}
}

func WithCloudAuditLogServiceName(serviceName string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Spec.ServiceName = serviceName
	}
}

func WithCloudAuditLogMethodName(methodName string) CloudAuditLogOption {
	return func(cal *v1alpha1.CloudAuditLog) {
		cal.Spec.MethodName = methodName
	}
}
