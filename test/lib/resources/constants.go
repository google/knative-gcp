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
	BatchAPIVersion             = "batch/v1"
	MessagingAPIVersion         = "messaging.cloud.google.com/v1alpha1"
	MessagingV1beta1APIVersion  = "messaging.cloud.google.com/v1beta1"
	EventsV1APIVersion          = "events.cloud.google.com/v1"
	EventsV1beta1APIVersion     = "events.cloud.google.com/v1beta1"
	EventsV1alpha1APIVersion    = "events.cloud.google.com/v1alpha1"
	IntEventsV1APIVersion       = "internal.events.cloud.google.com/v1"
	IntEventsV1beta1APIVersion  = "internal.events.cloud.google.com/v1beta1"
	IntEventsV1alpha1APIVersion = "internal.events.cloud.google.com/v1alpha1"
	ServingAPIVersion           = "serving.knative.dev/v1"
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
	// Increase from 40 seconds to 120 seconds, for issue: https://github.com/google/knative-gcp/issues/1568.
	// WaitDeletionTime for deleting resources
	WaitDeletionTime = 120 * time.Second
	// WaitCALTime for time needed to wait to fire an event after CAL Source is ready
	// Tried with 45 seconds but the test has been quite flaky.
	// Tried with 90 seconds but the test has been quite flaky.
	// Tried with 120 seconds but the test still has some flakiness.
	WaitCALTime = 150 * time.Second
	// WaitExtraSourceReadyTime for additional time needed to wait for a source becomes ready.
	WaitExtraSourceReadyTime = 90 * time.Second
	// As initially suspected in https://github.com/google/knative-gcp/issues/1437,
	// sometimes brokercell seems to take much longer than expected to reconcile
	// the broker config. Plus, the configmap propagation probably also takes a
	// little bit time.
	WaitBrokercellTime = 40 * time.Second
)
