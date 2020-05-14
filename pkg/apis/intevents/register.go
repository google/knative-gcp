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

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	GroupName = "internal.events.cloud.google.com"
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
