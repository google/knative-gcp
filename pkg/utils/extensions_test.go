/*
Copyright 2019 Google LLC

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

package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCEExtensions(t *testing.T) {
	extensions := map[string]string{
		"foo":   "bar",
		"boosh": "kakow",
	}
	extensionsString, _ := MapToBase64(extensions)
	// Test the to string
	{
		want := "eyJib29zaCI6Imtha293IiwiZm9vIjoiYmFyIn0="
		got := extensionsString
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
			t.Log(got)
		}
	}
	// Test the to map
	{
		want := extensions
		got, _ := Base64ToMap(extensionsString)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
			t.Log(got)
		}
	}
}
