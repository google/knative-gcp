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

package dataresidency

import (
	"cloud.google.com/go/pubsub"
)

// Defaults includes the default values to be populated by the Webhook.
type Defaults struct {
	// ClusterDefaults are the data residency defaults to use for all namepaces
	ClusterDefaults ScopedDefaults `json:"clusterDefaults,omitempty"`
}

// ScopedDefaults are the data residency setting defaults.
type ScopedDefaults struct {
	// AllowedPersistenceRegions specifies the regions allowed for data
	// storage. Eg "us-east1". An empty configuration means no data residency
	// constraints.
	AllowedPersistenceRegions []string `json:"messagestoragepolicy.allowedpersistenceregions,omitempty"`
}

// scoped gets the scoped data residency defaults, for now we only have
// cluster scope.
func (d *Defaults) scoped() *ScopedDefaults {
	scopedDefaults := &d.ClusterDefaults
	// currently we don't support namespace, but if we do, we should check
	// namespace default here.
	return scopedDefaults
}

// AllowedPersistenceRegions gets the AllowedPersistenceRegions setting in the default.
func (d *Defaults) AllowedPersistenceRegions() []string {
	return d.scoped().AllowedPersistenceRegions
}

// ComputeAllowedPersistenceRegions computes the final message storage policy in
// topicConfig. Return true if the topicConfig is updated.
func (d *Defaults) ComputeAllowedPersistenceRegions(topicConfig *pubsub.TopicConfig) bool {
	// We can do subset of both in the future, but for now, we just overwrite the
	// configuration as the relationship between region and zones are not clear to handle,
	// eg. us-east1 vs us-east1-a. Important note: setting the AllowedPersistenceRegions
	// to empty string slice is an error, should set it to nil for all regions.
	allowedRegions := d.AllowedPersistenceRegions()
	if allowedRegions == nil || len(allowedRegions) == 0 {
		return false
	}

	topicConfig.MessageStoragePolicy.AllowedPersistenceRegions = allowedRegions
	return true
}
