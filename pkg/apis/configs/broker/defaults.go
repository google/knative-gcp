/*
Copyright 2020 Google LLC.

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

package broker

import (
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	v1 "knative.dev/pkg/apis/duck/v1"
)

// Defaults includes the default values to be populated by the Webhook.
type Defaults struct {
	// NamespaceDefaults are the Broker delivery spec defaults to use in specific namespaces. The namespace is
	// the key, the value is the defaults.
	NamespaceDefaults map[string]ScopedDefaults `json:"namespaceDefaults,omitempty"`
	// ClusterDefaults are the Broker delivery spec defaults to use for all namepaces that are not in
	// NamespaceDefaults.
	ClusterDefaults ScopedDefaults `json:"clusterDefaults,omitempty"`
}

// ScopedDefaults are the Broker delivery setting defaults.
type ScopedDefaults struct {
	// ScopedDefaults implements the Broker delivery spec.
	*eventingduckv1beta1.DeliverySpec
}

// scoped gets the scoped Broker delivery setting defaults for the given namespace.
func (d *Defaults) scoped(ns string) *ScopedDefaults {
	scopedDefaults := &d.ClusterDefaults
	if sd, present := d.NamespaceDefaults[ns]; present {
		scopedDefaults = &sd
	}
	return scopedDefaults
}

func (d *Defaults) Retry(ns string) *int32 {
	sd := d.scoped(ns)
	return sd.Retry
}

func (d *Defaults) BackoffPolicy(ns string) *eventingduckv1beta1.BackoffPolicyType {
	sd := d.scoped(ns)
	return sd.BackoffPolicy
}

func (d *Defaults) BackoffDelay(ns string) *string {
	sd := d.scoped(ns)
	return sd.BackoffDelay
}

func (d *Defaults) DeadLetterSink(ns string) *v1.Destination {
	sd := d.scoped(ns)
	return sd.DeadLetterSink
}
