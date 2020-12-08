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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/labels"
	_ "knative.dev/pkg/system/testing"
)

func TestLabels(t *testing.T) {
	wantLabels := map[string]string{
		"app":        "cloud-run-events",
		"brokerCell": "default",
		"role":       "fanout",
	}
	gotLabels := Labels("default", "fanout")

	if diff := cmp.Diff(wantLabels, gotLabels); diff != "" {
		t.Fatal("Unexpected labels (-want, +got):", diff)
	}
}

func TestName(t *testing.T) {
	wantLabels := "default-brokercell-fanout"
	gotLabels := Name("default", "fanout")

	if diff := cmp.Diff(wantLabels, gotLabels); diff != "" {
		t.Fatal("Unexpected name (-want, +got):", diff)
	}
}

func TestGetLabelSelector(t *testing.T) {
	gotLabelSelector := GetLabelSelector("default", "fanout")
	wantLabelSelector := labels.SelectorFromSet(map[string]string{
		"app":        "cloud-run-events",
		"brokerCell": "default",
		"role":       "fanout",
	})
	if diff := cmp.Diff(gotLabelSelector.String(), wantLabelSelector.String()); diff != "" {
		t.Fatal("Unexpected labelSelector (-want, +got):", diff)
	}
}
