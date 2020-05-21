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

import (
	"github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"

	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
)

// Right now we onlyb support one brokercell in the system namespace in the cluster.
// TODO(#866) Delete hard-coded brokercell once we can dynamically assign brokercell to brokers.
const DefaultBroekrCellName = "default"

func CreateBrokerCell(b *v1beta1.Broker) *inteventsv1alpha1.BrokerCell {
	// TODO(#866) Get brokercell from the label (or annotation) from the broker.
	return &inteventsv1alpha1.BrokerCell{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      DefaultBroekrCellName,
			Annotations: map[string]string{inteventsv1alpha1.CreatorKey: inteventsv1alpha1.Creator},
		},
	}
}
