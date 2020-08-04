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

package v1

import (
	"time"

	"github.com/google/knative-gcp/pkg/reconciler/testing"

	"k8s.io/apimachinery/pkg/types"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type CloudAuditLogsSourceOption func(*v1.CloudAuditLogsSource)

func NewCloudAuditLogsSource(name, namespace string, opts ...CloudAuditLogsSourceOption) *v1.CloudAuditLogsSource {
	cal := &v1.CloudAuditLogsSource{
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

func WithInitCloudAuditLogsSourceConditions(s *v1.CloudAuditLogsSource) {
	s.Status.InitializeConditions()
}

func WithCloudAuditLogsSourceUID(uid string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.ObjectMeta.UID = types.UID(uid)
	}
}

func WithCloudAuditLogsSourceTopicFailed(reason, message string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkTopicFailed(s.ConditionSet(), reason, message)
	}
}

func WithCloudAuditLogsSourceWorkloadIdentityFailed(reason, message string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

func WithCloudAuditLogsSourceTopicUnknown(reason, message string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkTopicUnknown(s.ConditionSet(), reason, message)
	}
}

func WithCloudAuditLogsSourceTopicReady(topicID string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkTopicReady(s.ConditionSet())
		s.Status.TopicID = topicID
	}
}

func WithCloudAuditLogsSourcePullSubscriptionFailed(reason, message string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), reason, message)
	}
}

func WithCloudAuditLogsSourcePullSubscriptionUnknown(reason, message string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), reason, message)
	}
}

func WithCloudAuditLogsSourcePullSubscriptionReady() CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkPullSubscriptionReady(s.ConditionSet())
	}
}

func WithCloudAuditLogsSourceSinkNotReady(reason, messageFmt string, messageA ...interface{}) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkSinkNotReady(reason, messageFmt, messageA...)
	}
}

func WithCloudAuditLogsSourceSinkUnknown(reason, messageFmt string, messageA ...interface{}) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.MarkSinkUnknown(reason, messageFmt, messageA...)
	}
}

func WithCloudAuditLogsSourceSinkReady(s *v1.CloudAuditLogsSource) {
	s.Status.MarkSinkReady()
}

func WithCloudAuditLogsSourceSink(gvk metav1.GroupVersionKind, name string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudAuditLogsSourceSinkURI(url *apis.URL) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.SinkURI = url
	}
}

func WithCloudAuditLogsSourceProjectID(projectID string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.ProjectID = projectID
	}
}

func WithCloudAuditLogsSourceSubscriptionID(subscriptionID string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.SubscriptionID = subscriptionID
	}
}

func WithCloudAuditLogsSourceSinkID(sinkID string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Status.StackdriverSink = sinkID
	}
}

func WithCloudAuditLogsSourceProject(project string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Spec.Project = project
	}
}

func WithCloudAuditLogsSourceResourceName(resourceName string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Spec.ResourceName = resourceName
	}
}

func WithCloudAuditLogsSourceServiceName(serviceName string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Spec.ServiceName = serviceName
	}
}

func WithCloudAuditLogsSourceServiceAccount(kServiceAccount string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Spec.ServiceAccountName = kServiceAccount
	}
}

func WithCloudAuditLogsSourceMethodName(methodName string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Spec.MethodName = methodName
	}
}

func WithCloudAuditLogsSourceFinalizers(finalizers ...string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.Finalizers = finalizers
	}
}

func WithCloudAuditLogsSourceDeletionTimestamp(s *v1.CloudAuditLogsSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithCloudAuditLogsSourceAnnotations(Annotations map[string]string) CloudAuditLogsSourceOption {
	return func(s *v1.CloudAuditLogsSource) {
		s.ObjectMeta.Annotations = Annotations
	}
}

func WithCloudAuditLogsSourceSetDefaults(s *v1.CloudAuditLogsSource) {
	s.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
}
