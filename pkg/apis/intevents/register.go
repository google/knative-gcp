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

// Package intevents contains API versions for internal use by other
// resources.
package intevents

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "internal.events.cloud.google.com"

	// SourceLabelKey is the label name used to identify the source that owns a PS or Topic.
	SourceLabelKey = "events.cloud.google.com/source-name"
	// ChannelLabelKey is the label name used to identify the channel that owns a PS or Topic.
	ChannelLabelKey = "events.cloud.google.com/channel-name"
	// DefaultRetentionDuration is the default retention duration (7 days) in the default pullSubscription spec.
	DefaultRetentionDuration = 7 * 24 * time.Hour
	// DefaultAckDeadline is the default ack deadline (30 seconds) in the default pullSubscription spec.
	DefaultAckDeadline = 30 * time.Second
	// MinRetentionDuration is the minimum retention duration (10 seconds) to validate the pullSubscription.
	MinRetentionDuration = 10 * time.Second
	// MaxRetentionDuration is the maximum retention duration (7 days) to validate the pullSubscription.
	MaxRetentionDuration = 7 * 24 * time.Hour // 7 days.
	// MinAckDeadline is the minimum ack deadline (0 second) to validate the pullSubscription.
	MinAckDeadline = 0 * time.Second
	// MinAckDeadline is the maximum ack deadline (10 minutes) to validate the pullSubscription.
	MaxAckDeadline = 10 * time.Minute
)

var (
	// PullSubscriptionsResource represents a PullSubscription.
	PullSubscriptionsResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "pullsubscriptions",
	}
	// TopicsResource represents a Topic.
	TopicsResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "topics",
	}
)
