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

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// GenerateTopicName generates a topic name for the storage. This refers to the underlying Pub/Sub topic, and not our
// Topic resource.
func GenerateTopicName(storage *v1alpha1.CloudStorageSource) string {
	return fmt.Sprintf("storage-%s", string(storage.UID))
}

func GenerateSourceType() string {
	return "STORAGE"
}

func GenerateFilters(s *v1alpha1.CloudStorageSource) map[string]string {
	// TODO(nlopezgi):Need to figure out how to convert EventTypes properly
	return map[string]string{
		"Bucket":           s.Spec.Bucket,
		"EventTypes":       fmt.Sprintf("%v", s.Spec.EventTypes),
		"ObjectNamePrefix": s.Spec.ObjectNamePrefix,
		"PayloadFormat":    s.Spec.PayloadFormat,
	}
}
