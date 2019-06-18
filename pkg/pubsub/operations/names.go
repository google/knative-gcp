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

package operations

import (
	"strings"

	"github.com/knative/pkg/kmeta"
)

// TopicJobName creates the name of a topic ops job.
func TopicJobName(owner kmeta.OwnerRefable, action string) string {
	return strings.ToLower(
		strings.Join(append([]string{
			"pubsub",
			"t",
			owner.GetObjectMeta().GetName(),
			owner.GetGroupVersionKind().Kind,
			action}),
			"-") + "-",
	)
}

// SubscriptionJobName creates the name of a subscription ops job.
func SubscriptionJobName(owner kmeta.OwnerRefable, action string) string {
	return strings.ToLower(
		strings.Join(append([]string{
			"pubsub",
			"s",
			owner.GetObjectMeta().GetName(),
			owner.GetGroupVersionKind().Kind,
			action}),
			"-") + "-",
	)
}
