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

	"knative.dev/pkg/kmeta"
)

// TopicJobLabels creates a label to find a job again.
// keys is recommended to be (name, kind, action)
func TopicJobLabels(owner kmeta.OwnerRefable, action string) map[string]string {
	return jobLabels("topic", owner.GetObjectMeta().GetName(), owner.GetGroupVersionKind().Kind, action)
}

func SubscriptionJobLabels(owner kmeta.OwnerRefable, action string) map[string]string {
	return jobLabels("subscription", owner.GetObjectMeta().GetName(), owner.GetGroupVersionKind().Kind, action)
}

// SubscriptionJobLabels creates a label to find a job again.
// keys is recommended to be (name, kind, action)
func jobLabels(key string, parts ...string) map[string]string {
	return map[string]string{
		"pubsub.cloud.run/" + key: strings.Join(append([]string{"ops"}, parts...), "-"),
	}
}
