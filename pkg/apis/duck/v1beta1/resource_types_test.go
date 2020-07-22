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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResource_GetFullType(t *testing.T) {
	s := &Resource{}
	switch s.GetFullType().(type) {
	case *Resource:
		// expected
	default:
		t.Errorf("expected GetFullType to return *Resource, got %T", s.GetFullType())
	}
}

func TestResource_GetListType(t *testing.T) {
	s := &Resource{}
	switch s.GetListType().(type) {
	case *ResourceList:
		// expected
	default:
		t.Errorf("expected GetListType to return *ResourceList, got %T", s.GetListType())
	}
}

func TestResource_Populate(t *testing.T) {
	got := &Resource{}

	want := &Resource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1beta1",
			Kind:       "MyKind",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}
}
