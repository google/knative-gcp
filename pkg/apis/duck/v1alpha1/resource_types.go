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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
)

// GroupResource is an Implementable "duck type".
var _ duck.Implementable = (*Resource)(nil)

// +genduck
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupResource is a skeleton type wrapping all Kubernetes resources. It is typically used to watch
// arbitrary other resources. This is not a real resource.
// TODO upstream to pkg
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

var (
	// Verify GroupResource resources meet duck contracts.
	_ duck.Populatable = (*Resource)(nil)
	_ apis.Listable    = (*Resource)(nil)
)

// GetFullType implements duck.Implementable
func (*Resource) GetFullType() duck.Populatable {
	return &Resource{}
}

// Populate implements duck.Populatable
func (s *Resource) Populate() {
	s.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1alpha1",
		Kind:       "MyKind",
	}
	s.ObjectMeta = metav1.ObjectMeta{
		Name:      "foo",
		Namespace: "bar",
	}
}

// GetListType implements apis.Listable
func (*Resource) GetListType() runtime.Object {
	return &ResourceList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceList is a list of GroupResource resources
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Resource `json:"items"`
}
