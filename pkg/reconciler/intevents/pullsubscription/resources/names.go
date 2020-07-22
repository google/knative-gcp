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

	"github.com/google/knative-gcp/pkg/apis/intevents"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/utils/naming"
	"knative.dev/pkg/kmeta"
)

// GenerateSubscriptionName generates the name for the Pub/Sub subscription to be used for this PullSubscription.
//  It uses the object labels to see whether it's from a source, channel, or ps to construct the name.
func GenerateSubscriptionName(ps *v1.PullSubscription) string {
	prefix := getPrefix(ps)
	return naming.TruncatedPubsubResourceName(prefix, ps.Namespace, ps.Name, ps.UID)
}

// GenerateReceiveAdapterName generates the name of the receive adapter to be used for this PullSubscription.
func GenerateReceiveAdapterName(ps *v1.PullSubscription) string {
	return GenerateK8sName(ps)
}

// GenerateK8sName generates a k8s name based on PullSubscription information.
//  It uses the object labels to see whether it's from a source, channel, or ps to constructs a k8s compliant name.
func GenerateK8sName(ps *v1.PullSubscription) string {
	prefix := getPrefix(ps)
	return kmeta.ChildName(fmt.Sprintf("%s-%s", prefix, ps.Name), "-"+string(ps.UID))
}

func getPrefix(ps *v1.PullSubscription) string {
	prefix := "cre-ps"
	if _, ok := ps.Labels[intevents.SourceLabelKey]; ok {
		prefix = "cre-src"
	} else if _, ok := ps.Labels[intevents.ChannelLabelKey]; ok {
		prefix = "cre-chan"
	}
	return prefix
}
