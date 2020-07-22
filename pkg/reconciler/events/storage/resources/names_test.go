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
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateTopicName(t *testing.T) {
	want := "cre-src_mynamespace_myname_uid"
	got := GenerateTopicName(&v1.CloudStorageSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "mynamespace",
			UID:       "uid",
		},
	})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
