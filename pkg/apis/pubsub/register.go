/*
Copyright 2019 Google LLC

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

// Package pubsub contains Cloud Run Events API versions for pubsub components
package pubsub

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	GroupName = "pubsub.cloud.google.com"
)

var (
	// PullSubscriptionsResource represents a PubSub PullSubscription.
	PullSubscriptionsResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "pullsubscriptions",
	}
	// TopicsResource represents a PubSub Topic.
	TopicsResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "topics",
	}
)
