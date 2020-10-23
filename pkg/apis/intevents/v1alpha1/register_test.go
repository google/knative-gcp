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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestKind(t *testing.T) {
	for n, tc := range map[string]struct {
		kind string
	}{
		"BrokerCell": {
			kind: "BrokerCell",
		},
	} {
		t.Run(n, func(t *testing.T) {
			want := schema.GroupKind{
				Group: "internal.events.cloud.google.com",
				Kind:  tc.kind,
			}
			got := Kind(tc.kind)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(Kind (-want +got): %v", diff)
			}
		})
	}
}

func TestResource(t *testing.T) {
	for n, tc := range map[string]struct {
		resource string
	}{
		"BrokerCell": {
			resource: "BrokerCell",
		},
	} {
		t.Run(n, func(t *testing.T) {
			want := schema.GroupResource{
				Group:    "internal.events.cloud.google.com",
				Resource: tc.resource,
			}
			got := Resource(tc.resource)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(Kind (-want +got): %v", diff)
			}
		})
	}
}

func TestAddKnownTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := addKnownTypes(scheme); err != nil {
		t.Errorf("error in addKnownTypes: %w", err)
	}

	want := []string{
		"BrokerCell",
		"BrokerCellList",
	}
	got := scheme.KnownTypes(schema.GroupVersion{Group: "internal.events.cloud.google.com", Version: "v1alpha1"})

	for _, tn := range want {
		if _, exist := got[tn]; !exist {
			t.Errorf("type %s doesn't exist in scheme", tn)
		}
	}
}
