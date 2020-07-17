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

package v1beta1

import (
	"testing"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestChannelDefaults(t *testing.T) {
	for n, tc := range map[string]struct {
		in   Channel
		want Channel
	}{
		"no subscribable annotation": {
			in: Channel{},
			want: Channel{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"messaging.knative.dev/subscribable": "v1beta1",
					},
				},
				Spec: ChannelSpec{
					Secret: &gcpauthtesthelper.Secret,
				},
			},
		},
		"with a subscribable annotation": {
			in: Channel{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"messaging.knative.dev/subscribable": "v1beta1",
					},
				},
			},
			want: Channel{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"messaging.knative.dev/subscribable": "v1beta1",
					},
				},
				Spec: ChannelSpec{
					Secret: &gcpauthtesthelper.Secret,
				},
			},
		},
	} {
		t.Run(n, func(t *testing.T) {
			got := tc.in
			got.SetDefaults(gcpauthtesthelper.ContextWithDefaults())

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}
