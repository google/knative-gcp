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

package gcpauth

import (
	corev1 "k8s.io/api/core/v1"
)

// Defaults includes the default values to be populated by the Webhook.
type Defaults struct {
	// NamespaceDefaults are the GCP auth defaults to use in specific namespaces. The namespace is
	// the key, the value is the defaults.
	NamespaceDefaults map[string]ScopedDefaults `json:"namespaceDefaults,omitempty"`
	// ClusterDefaults are the GCP auth defaults to use for all namepaces that are not in
	// NamespaceDefaults.
	ClusterDefaults ScopedDefaults `json:"clusterDefaults,omitempty"`
}

// ScopedDefaults are the GCP auth defaults.
type ScopedDefaults struct {
	// ServiceAccountName is the Kubernetes Service Account to user for all data plane pieces. This
	// is expected to be used for Workload Identity workloads.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Secret is the secret to default to, if one is not already in the CO's spec.
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// WorkloadIdentityMapping is a mapping from Kubernetes Service Account to Google IAM Service
	// Account. If a GCP authable's spec.ServiceAccountName is in this map, then the controller will
	// attempt to setup Workload Identity between the two accounts. If it is unable to do so, then
	// the CO will not become ready.
	WorkloadIdentityMapping map[string]string `json:"workloadIdentityMapping,omitEmpty"`
}

// scoped gets the scoped GCP Auth defaults for the given namespace.
func (d *Defaults) scoped(ns string) *ScopedDefaults {
	scopedDefaults := &d.ClusterDefaults
	if sd, present := d.NamespaceDefaults[ns]; present {
		scopedDefaults = &sd
	}
	return scopedDefaults
}

func (d *Defaults) KSA(ns string) string {
	sd := d.scoped(ns)
	return sd.ServiceAccountName
}

func (d *Defaults) Secret(ns string) *corev1.SecretKeySelector {
	sd := d.scoped(ns)
	return sd.Secret
}

func (d *Defaults) WorkloadIdentityGSA(ns, ksa string) string {
	sd := d.scoped(ns)
	return sd.WorkloadIdentityMapping[ksa]
}
