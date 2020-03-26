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

package v1alpha1

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestIdentityGetListType(t *testing.T) {
	c := &Identity{}
	switch c.GetListType().(type) {
	case *IdentityList:
		// expected
	default:
		t.Errorf("expected GetListType to return *ChannelableList, got %T", c.GetListType())
	}
}

func TestIdentityPopulate(t *testing.T) {
	got := &Identity{}
	want := &Identity{
		Spec: IdentitySpec{},
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}
}

func TestGetFullType(t *testing.T) {
	got := &Identity{}
	want := &Identity{}

	got.GetFullType()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}
}

func TestIsReady(t *testing.T) {
	status := &IdentityStatus{}
	want := false

	got := status.IsReady()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}
}
