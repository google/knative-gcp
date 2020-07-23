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
	"github.com/google/knative-gcp/pkg/apis/intevents"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateScaledObjectName(t *testing.T) {
	tests := []struct {
		name string
		ps   *v1.PullSubscription
		want string
	}{{
		name: "ps-based name",
		ps: &v1.PullSubscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myname",
				Namespace: "mynamespace",
				UID:       "uid",
			},
		},
		want: "cre-ps-myname-uid",
	}, {
		name: "source-based name",
		ps: &v1.PullSubscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myname",
				Namespace: "mynamespace",
				UID:       "uid",
				Labels: map[string]string{
					intevents.SourceLabelKey: "myname",
				},
			},
		},
		want: "cre-src-myname-uid",
	}, {
		name: "channel-based name",
		ps: &v1.PullSubscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myname",
				Namespace: "mynamespace",
				UID:       "uid",
				Labels: map[string]string{
					intevents.ChannelLabelKey: "myname",
				},
			},
		},
		want: "cre-chan-myname-uid",
	}, {
		name: "name too long, hashed and shortened",
		ps: &v1.PullSubscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mynameistoooooolongggggggggggggggggggggggggggggggggg",
				Namespace: "mynamespace",
				UID:       "uid",
				Labels: map[string]string{
					intevents.ChannelLabelKey: "myname",
				},
			},
		},
		want: "cre-chan-mynameistoooooolon597d25200bc5966cf43c8847aba50757-uid",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GenerateScaledObjectName(test.ps)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected (-want, +got) = %v", diff)
			}
		})
	}
}
