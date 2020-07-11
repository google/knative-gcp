/*
Copyright 2020 Google LLC

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

package resources

import "time"

// API versions for the resources.
const (
	BatchAPIVersion            = "batch/v1"
	MessagingAPIVersion        = "messaging.cloud.google.com/v1alpha1"
	MessagingV1beta1APIVersion = "messaging.cloud.google.com/v1beta1"
	EventsAPIVersion           = "events.cloud.google.com/v1alpha1"
	IntEventsAPIVersion        = "internal.events.cloud.google.com/v1alpha1"
	ServingAPIVersion          = "serving.knative.dev/v1"
)

// Kind for batch resources.
const (
	JobKind string = "Job"
)

// Kind for messaging resources.
const (
	ChannelKind string = "Channel"
)

// Kind for events resources.
const (
	CloudStorageSourceKind   string = "CloudStorageSource"
	CloudPubSubSourceKind    string = "CloudPubSubSource"
	CloudBuildSourceKind     string = "CloudBuildSource"
	CloudAuditLogsSourceKind string = "CloudAuditLogsSource"
	CloudSchedulerSourceKind string = "CloudSchedulerSource"
)

// Kind for pubsub resources.
const (
	PullSubscriptionKind string = "PullSubscription"
)

// Kind for Knative resources.
const (
	KServiceKind string = "Service"
)

const (
	// Tried with 20 seconds but the test has been quite flaky.
	// WaitDeletionTime for deleting resources
	WaitDeletionTime = 40 * time.Second
	// WaitCALTime for time needed to wait to fire an event after CAL Source is ready
	// Tried with 45 seconds but the test has been quite flaky.
	// Tried with 90 seconds but the test has been quite flaky.
	WaitCALTime = 120 * time.Second
)
