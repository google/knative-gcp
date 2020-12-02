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
	"context"
)

const (
	knativeServingV1ApiVersion       = "serving.knative.dev/v1"
	knativeServingV1Alpha1ApiVersion = "serving.knative.dev/v1alpha1"
)

// SetDefaults sets the default field values for a Trigger.
func (t *Trigger) SetDefaults(ctx context.Context) {
	// This upgrades the `ApiVersion` of the knative serving subscriber form v1alpha1 to v1, this is intended to
	// help users who are lagging in their transition to knative serving v1.
	if t.Spec.Subscriber.Ref != nil && t.Spec.Subscriber.Ref.APIVersion == knativeServingV1Alpha1ApiVersion {
		t.Spec.Subscriber.Ref.APIVersion = knativeServingV1ApiVersion
	}
}
