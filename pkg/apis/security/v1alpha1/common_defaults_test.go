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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestJWTDefaults(t *testing.T) {
	j := &JWTSpec{}
	j.SetDefaults(context.Background())
	want := []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}}
	if diff := cmp.Diff(want, j.FromHeaders); diff != "" {
		t.Errorf("default FromHeaders (-want, +got) = %v", diff)
	}
}

func TestPolicyBindingSpecDefaults(t *testing.T) {
	spec := &PolicyBindingSpec{Policy: duckv1.KReference{}}
	spec.SetDefaults(context.Background(), "test-namespace")
	if spec.Subject.Namespace != "test-namespace" {
		t.Errorf("spec.Subject.Namespace got=%s want=test-namespace", spec.Subject.Namespace)
	}
	if spec.Policy.Namespace != "test-namespace" {
		t.Errorf("spec.Policy.Namespace got=%s want=test-namespace", spec.Policy.Namespace)
	}
}
