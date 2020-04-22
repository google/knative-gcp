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

// PubSubTriggerOption enables further configuration of a Trigger.
type PubSubTriggerOption func(*v1beta1.Trigger)

// NewPubSubTrigger creates a Trigger with TriggerOptions
func NewPubSubTrigger(name, namespace string, so ...PubSubTriggerOption) *v1beta1.Trigger {
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

/*func WithPubSubTriggerProject(project string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.Project = project
	}
}*/

// WithInitTriggerConditions initializes the Triggers's conditions.
func WithInitPubSubTriggerConditions(s *v1beta1.Trigger) {
	s.Status.InitializeConditions()
}

// WithPubSubTriggerServiceAccountName will give status.ServiceAccountName a k8s service account name, which is related on Workload Identity's Google service account.
func WithPubSubTriggerServiceAccountName(name string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.ServiceAccountName = name
	}
}

func WithPubSubTriggerWorkloadIdentityFailed(reason, message string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

func WithPubSubTriggerGCPServiceAccount(gServiceAccount string) PubSubTriggerOption {
	return func(ps *v1beta1.Trigger) {
		ps.Spec.IdentitySpec.GoogleServiceAccount = gServiceAccount
	}
}

// WithPubSubTriggerTriggerNotReady marks the condition that the
// GCS Trigger is not ready.
func WithPubSubTriggerTriggerNotReady(reason, message string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.MarkTriggerNotReady(reason, message)
	}
}

// WithPubSubTriggerTriggerReady marks the condition that the GCS
// Trigger is ready.
func WithPubSubTriggerTriggerReady(triggerID string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.MarkTriggerReady(triggerID)
	}
}

// WithPubSubTriggerTriggerId sets the status for Trigger ID
func WithPubSubTriggerTriggerID(triggerID string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.TriggerID = triggerID
	}
}

// WithPubSubTriggerProjectId sets the status for Project ID
func WithPubSubTriggerProjectID(projectID string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.ProjectID = projectID
	}
}

func WithPubSubTriggerStatusObservedGeneration(generation int64) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Status.Status.ObservedGeneration = generation
	}
}

func WithPubSubTriggerObjectMetaGeneration(generation int64) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.ObjectMeta.Generation = generation
	}
}

/*
func WithPubSubTriggerSourceType(sourceType string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.SourceType = sourceType
	}
}

func WithPubSubTriggerTrigger(trigger string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.Trigger = trigger
	}
}*/

func WithPubSubTriggerSpec(spec v1beta1.TriggerSpec) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec = spec
	}
}

/*func WithPubSubTriggerFilter(key string, value string) PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		s.Spec.Filters[key] = value
	}
}*/

func WithPubSubTriggerDeletionTimestamp() PubSubTriggerOption {
	return func(s *v1beta1.Trigger) {
		ts := metav1.NewTime(time.Unix(1e9, 0))
		s.DeletionTimestamp = &ts
	}
}
