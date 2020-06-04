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

package resources

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/kmeta"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	subscriptionNamePrefix = "cre-sub-"
)

// GenerateTopicID generates the name of the Pub/Sub topic, not our Topic resource.
func GenerateTopicID(channel *v1alpha1.Channel) string {
	return utils.TruncatedPubsubResourceName("cre-chan", channel.Namespace, channel.Name, channel.UID)
}

func GeneratePublisherName(channel *v1alpha1.Channel) string {
	if strings.HasPrefix(channel.Name, "cre-") {
		return kmeta.ChildName(channel.Name, "-chan")
	}
	return kmeta.ChildName(fmt.Sprintf("cre-%s", channel.Name), "-chan")
}

// GeneratePullSubscriptionName generates the name of the PullSubscription resource using the subscriber's UID.
func GeneratePullSubscriptionName(UID types.UID) string {
	return fmt.Sprintf("%s%s", subscriptionNamePrefix, string(UID))
}

// ExtractUIDFromPullSubscriptionName extracts the subscriber's UID from the PullSubscription name.
func ExtractUIDFromPullSubscriptionName(name string) string {
	return strings.TrimPrefix(name, subscriptionNamePrefix)
}
