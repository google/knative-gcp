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

package v1

import (
	"testing"
)

func TestCloudStorageEventSource(t *testing.T) {
	want := "//storage.googleapis.com/buckets/bucket"
	got := CloudStorageEventSource("bucket")
	if got != want {
		t.Errorf("CloudStorageEventSource got=%s, want=%s", got, want)
	}
}

func TestCloudStorageEventSubject(t *testing.T) {
	want := "objects/obj"
	got := CloudStorageEventSubject("obj")
	if got != want {
		t.Errorf("CloudStorageEventSubject got=%s, want=%s", got, want)
	}
}
