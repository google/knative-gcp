/*
Copyright 2020 The Knative Authors

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*v1beta1.Trigger)

// NewTrigger creates a Trigger with TriggerOptions
func NewTrigger(name, namespace string, so ...TriggerOption) *v1beta1.Trigger {
	s := &v1beta1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-trigger-uid",
		},
	}
	s.Spec.Filters = map[string]string{}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithTriggerProject(project string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.Project = project
	}
}

// WithInitTriggerConditions initializes the Triggers's conditions.
func WithInitTriggerConditions(s *v1beta1.Trigger) {
	s.Status.InitializeConditions()
}

// WithTriggerServiceAccountName will give status.ServiceAccountName a k8s service account name, which is related on Workload Identity's Google service account.
func WithTriggerServiceAccountName(name string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.ServiceAccountName = name
	}
}

func WithTriggerWorkloadIdentityFailed(reason, message string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

func WithTriggerGCPServiceAccount(gServiceAccount string) TriggerOption {
	return func(ps *v1beta1.Trigger) {
		ps.Spec.GoogleServiceAccount = gServiceAccount
	}
}

// WithTriggerTriggerNotReady marks the condition that the
// GCS Trigger is not ready.
func WithTriggerTriggerNotReady(reason, message string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.MarkTriggerNotReady(reason, message)
	}
}

// WithTriggerTriggerReady marks the condition that the GCS
// Trigger is ready.
func WithTriggerTriggerReady(triggerID string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.MarkTriggerReady(triggerID)
	}
}

// WithTriggerTriggerId sets the status for Trigger ID
func WithTriggerTriggerID(triggerID string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.TriggerID = triggerID
	}
}

// WithTriggerProjectId sets the status for Project ID
func WithTriggerProjectID(projectID string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.ProjectID = projectID
	}
}

func WithTriggerStatusObservedGeneration(generation int64) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.Status.ObservedGeneration = generation
	}
}

func WithTriggerObjectMetaGeneration(generation int64) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.ObjectMeta.Generation = generation
	}
}

func WithTriggerSourceType(sourceType string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.SourceType = sourceType
	}
}

func WithTriggerFilter(key string, value string) TriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.Filters[key] = value
	}
}

func WithTriggerDeletionTimestamp() TriggerOption {
	return func(s *v1beta1.Trigger) {
		ts := metav1.NewTime(time.Unix(1e9, 0))
		s.DeletionTimestamp = &ts
	}
}
