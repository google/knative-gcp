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

package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupResource takes an unqualified resource and returns a Group qualified GroupResource
func TestGroupResource(t *testing.T) {
	want := schema.GroupResource{
		Group:    "events.cloud.google.com",
		Resource: "foo",
	}

	got := Resource("foo")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected resource (-want, +got) = %v", diff)
	}
}

func TestKind(t *testing.T) {
	want := schema.GroupKind{
		Group: "events.cloud.google.com",
		Kind:  "foo",
	}

	got := Kind("foo")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected resource (-want, +got) = %v", diff)
	}
}

// TestKnownTypes makes sure that expected types get added.
func TestKnownTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	addKnownTypes(scheme)
	types := scheme.KnownTypes(SchemeGroupVersion)

	for _, name := range []string{
		"CloudAuditLogsSource",
		"CloudBuildSource",
		"CloudPubSubSource",
		"CloudSchedulerSource",
		"CloudStorageSource",
	} {
		if _, ok := types[name]; !ok {
			t.Errorf("Did not find %q as registered type", name)
		}
	}

}
