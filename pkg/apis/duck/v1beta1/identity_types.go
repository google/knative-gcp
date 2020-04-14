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

package v1beta1

import (
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type IdentitySpec struct {
	// GoogleServiceAccount is the GCP service account which has required permissions to poll from a Cloud Pub/Sub subscription.
	// If not specified, defaults to use secret.
	// +optional
	GoogleServiceAccount string `json:"googleServiceAccount,omitempty"`
}

// IdentityStatus inherits duck/v1 Status and adds a ServiceAccountName.
type IdentityStatus struct {
	// Inherits duck/v1 Status,, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
	// ServiceAccountName is the k8s service account associated with Google service account.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	IdentityConfigured apis.ConditionType = "WorkloadIdentityConfigured"
)

// IsReady returns true if the resource is ready overall.
func (ss *IdentityStatus) IsReady() bool {
	for _, c := range ss.Conditions {
		switch c.Type {
		// Look for the "happy" condition, which is the only condition that
		// we can reliably understand to be the overall state of the resource.
		case apis.ConditionReady, apis.ConditionSucceeded:
			return c.IsTrue()
		}
	}
	return false
}
