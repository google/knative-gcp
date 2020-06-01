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

package authorization

import (
	corev1 "k8s.io/api/core/v1"
)

// Defaults includes the default values to be populated by the webhook.
type Defaults struct {
	// NamespaceDefaults are the default Authorizations CRDs for each namespace. namespace is the
	// key, the value is the default AuthorizationTemplate to use.
	NamespaceDefaults map[string]ScopedDefaults `json:"namespaceDefaults,omitempty"`
	// ClusterDefaults is the default Authorization CRD for all namespaces that are not in
	// NamespaceDefaults.
	ClusterDefaults ScopedDefaults `json:"clusterDefaults,omitempty"`
}

type ScopedDefaults struct {
	ServiceAccountName      string                    `json:"serviceAccountName,omitempty"`
	Secret                  *corev1.SecretKeySelector `json:"secret,omitempty"`
	WorkloadIdentityMapping map[string]string         `json:"workloadIdentityMapping,omitEmpty"`
}

// scoped gets the scoped Authorization defaults for the given namespace.
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
