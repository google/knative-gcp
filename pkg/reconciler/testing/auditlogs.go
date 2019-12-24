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
	"time"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type AuditLogsSourceOption func(*v1alpha1.AuditLogsSource)

func NewAuditLogsSource(name, namespace string, opts ...AuditLogsSourceOption) *v1alpha1.AuditLogsSource {
	cal := &v1alpha1.AuditLogsSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-auditlogssource-uid",
		},
	}
	for _, opt := range opts {
		opt(cal)
	}
	cal.SetDefaults(context.Background())
	return cal
}

func WithInitAuditLogsSourceConditions(s *v1alpha1.AuditLogsSource) {
	s.Status.InitializeConditions()
}

func WithAuditLogsSourceTopicNotReady(reason, message string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.MarkTopicNotReady(reason, message)
	}
}

func WithAuditLogsSourceTopicReady(topicID string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

func WithAuditLogsSourcePullSubscriptionNotReady(reason, message string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.MarkPullSubscriptionNotReady(reason, message)
	}
}

func WithAuditLogsSourcePullSubscriptionReady() AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.MarkPullSubscriptionReady()
	}
}

func WithAuditLogsSourceSinkNotReady(reason, messageFmt string, messageA ...interface{}) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.MarkSinkNotReady(reason, messageFmt, messageA...)
	}
}

func WithAuditLogsSourceSinkReady() AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.MarkSinkReady()
	}
}

func WithAuditLogsSourceSink(gvk metav1.GroupVersionKind, name string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithAuditLogsSourceSinkURI(url *apis.URL) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.SinkURI = url
	}
}

func WithAuditLogsSourceProjectID(projectID string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.ProjectID = projectID
	}
}

func WithAuditLogsSourceSinkID(sinkID string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Status.SinkID = sinkID
	}
}

func WithAuditLogsSourceServiceName(serviceName string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Spec.ServiceName = serviceName
	}
}

func WithAuditLogsSourceMethodName(methodName string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Spec.MethodName = methodName
	}
}

func WithAuditLogsSourceFinalizers(finalizers ...string) AuditLogsSourceOption {
	return func(s *v1alpha1.AuditLogsSource) {
		s.Finalizers = finalizers
	}
}

func WithAuditLogsSourceDeletionTimestamp(s *v1alpha1.AuditLogsSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}
