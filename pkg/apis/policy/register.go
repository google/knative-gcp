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

// Package policy contains experimental policy API definitions.
package policy

const (
	// GroupName is the API group name.
	GroupName = "policy.run.cloud.google.com"

	// PolicyBindingClassAnnotationKey is the annotation key for policy binding class.
	PolicyBindingClassAnnotationKey = GroupName + "/policybinding-class"

	// AuthorizableAnnotationKey is the annotaion key for Authorizables.
	AuthorizableAnnotationKey = GroupName + "/authorizableOn"

	// SelfAuthorizableAnnotationValue is the annotation value if an object itself is an Authorizable.
	SelfAuthorizableAnnotationValue = "self"

	// IstioPolicyBindingClassValue is the binding class name for Istio implementation.
	IstioPolicyBindingClassValue = "istio"
)
