/*
Copyright 2019 The Knative Authors

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

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

func TestChannelDefaults(t *testing.T) {
	want := &Channel{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"messaging.knative.dev/subscribable": "v1alpha1",
				duckv1alpha1.ClusterNameAnnotation:   testingMetadataClient.FakeClusterName,
			},
		},
		Spec: ChannelSpec{
			Secret: &gcpauthtesthelper.Secret,
		}}

	got := &Channel{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
			},
		},
		Spec: ChannelSpec{},
	}
	got.SetDefaults(gcpauthtesthelper.ContextWithDefaults())

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
