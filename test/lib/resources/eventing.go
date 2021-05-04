/*
Copyright 2021 Google LLC

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
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingresources "knative.dev/eventing/test/lib/resources"
)

// WithDependencyAnnotationTriggerV1 returns an option that adds a dependency annotation to the given V1 Trigger.
// This option is missing from eventing v1 test resources, so we must patch it in ourselves.
func WithDependencyAnnotationTriggerV1(dependencyAnnotation string) eventingresources.TriggerOptionV1 {
	return func(t *eventingv1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[eventingv1.DependencyAnnotation] = dependencyAnnotation
	}
}
