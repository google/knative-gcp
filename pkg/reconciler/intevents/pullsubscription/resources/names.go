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
	"github.com/google/knative-gcp/pkg/apis/intevents"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
)

// GenerateSubscriptionName generates the name for the Pub/Sub subscription to be used for this PullSubscription.
func GenerateSubscriptionName(ps *v1alpha1.PullSubscription) string {
	return intevents.GenerateName(ps)
}

// GenerateReceiveAdapterName generates the name for the receive adapter to be used for this PullSubscription.
func GenerateReceiveAdapterName(ps *v1alpha1.PullSubscription) string {
	return intevents.GenerateK8sName(ps)
}
