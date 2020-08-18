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

package customresource

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// RetrieveLatestUpdateTime takes an arbitrary custom resource and retrives its latest update time
// based on the resource's metadata
func RetrieveLatestUpdateTime(customResource metav1.ObjectMetaAccessor) (time.Time, error) {
	resourceKR, krAssertable := customResource.(duckv1.KRShaped)
	if !krAssertable {
		return time.Time{}, fmt.Errorf("The specified customResource does not support an expected type assertion (KRShaped: %v) %#v", krAssertable, customResource)
	}
	latestUpdateTime := customResource.GetObjectMeta().GetCreationTimestamp()
	if customResource.GetObjectMeta().GetDeletionTimestamp() != nil {
		latestUpdateTime = *customResource.GetObjectMeta().GetDeletionTimestamp()
	} else {
		status := resourceKR.GetStatus()
		if status != nil && status.Conditions != nil {
			for _, cond := range status.Conditions {
				if latestUpdateTime.Before(&cond.LastTransitionTime.Inner) {
					latestUpdateTime = cond.LastTransitionTime.Inner
				}
			}
		}
	}
	return latestUpdateTime.Time, nil
}
