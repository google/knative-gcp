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
	"context"
	"net/url"
	"testing"

	"knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// These variables are used to create a 'complete' version of Topic where every field is filled in.
var (
	completeAddressStatus = v1alpha1.AddressStatus{
		Address: &v1alpha1.Addressable{
			Addressable: duckv1beta1.Addressable{
				URL: &completeURL,
			},
			Hostname: completeURL.Host,
		},
	}

	// completeTopic is a Topic with every filled in, except TypeMeta. TypeMeta is excluded because
	// conversions do not convert it and this variable was created to test conversions.
	completeTopic = &Topic{
		ObjectMeta: completeObjectMeta,
		Spec: TopicSpec{
			IdentitySpec:      completeIdentitySpec,
			Secret:            completeSecret,
			Project:           "project",
			Topic:             "topic",
			PropagationPolicy: TopicPolicyCreateDelete,
		},
		Status: TopicStatus{
			IdentityStatus: completeIdentityStatus,
			AddressStatus:  completeAddressStatus,
			ProjectID:      "projectID",
			TopicID:        "topicID",
		},
	}
)

func TestTopicConversionBadType(t *testing.T) {
	good, bad := &Topic{}, &PullSubscription{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestTopicConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.Topic{}}

	tests := []struct {
		name string
		in   *Topic
	}{{
		name: "min configuration",
		in: &Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "topic-name",
				Namespace:  "topic-ns",
				Generation: 17,
			},
			Spec: TopicSpec{},
		},
	}, {
		name: "hostname, no URL",
		in: &Topic{
			ObjectMeta: completeObjectMeta,
			Status: TopicStatus{
				AddressStatus: v1alpha1.AddressStatus{
					Address: &v1alpha1.Addressable{
						Hostname: "not-in-uri",
					},
				},
			},
		},
	}, {
		name: "full configuration",
		in:   completeTopic,
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Topic{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				// Hostname exists in v1alpha1, but not in v1beta1, so it will not be present after
				// the round-trip conversion. Compare it manually, then overwrite the output value
				// so that the generic comparison later works.
				if test.in.Status.Address != nil {
					if wantHostname := test.in.Status.Address.Hostname; wantHostname != "" {
						if gotHostname := got.Status.Address.URL.Host; wantHostname != gotHostname {
							// Note that this assumes that if both a URL and a hostname are
							// specified in the original, then they have the same value.
							t.Errorf("Incorrect hostname, want %q, got %q", wantHostname, gotHostname)
						}
						got.Status.Address = test.in.Status.Address
					}
				}

				ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
				if diff := cmp.Diff(test.in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
