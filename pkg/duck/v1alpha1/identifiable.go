/*
Copyright 2020 The Knative Authors

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

package duck

import (
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

type Identifiable interface {
	kmeta.OwnerRefable
	// IdentitySpec returns the IdentitySpec portion of the Spec.
	IdentitySpec() *duckv1alpha1.IdentitySpec
	// IdentityStatus returns the IdentityStatus portion of the Status.
	IdentityStatus() *duckv1alpha1.IdentityStatus
	// ConditionSet returns the apis.ConditionSet of the embedding object
	ConditionSet() *apis.ConditionSet
}
