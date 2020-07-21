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

package v1beta1

import (
	"context"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	gcptesting "github.com/google/knative-gcp/pkg/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
)

// These variables are used to create a 'complete' version of Topic where every field is filled in.
var (
	// completeTopic is a Topic with every filled in, except TypeMeta. TypeMeta is excluded because
	// conversions do not convert it and this variable was created to test conversions.
	completeTopic = &Topic{
		ObjectMeta: gcptesting.CompleteObjectMeta,
		Spec: TopicSpec{
			IdentitySpec:      gcptesting.CompleteV1beta1IdentitySpec,
			Secret:            gcptesting.CompleteSecret,
			Project:           "project",
			Topic:             "topic",
			PropagationPolicy: TopicPolicyCreateDelete,
			EnablePublisher:   &trueVal,
		},
		Status: TopicStatus{
			IdentityStatus: gcptesting.CompleteV1beta1IdentityStatus,
			AddressStatus:  gcptesting.CompleteV1beta1AddressStatus,
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
	versions := []apis.Convertible{&v1.Topic{}}

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
			ObjectMeta: gcptesting.CompleteObjectMeta,
			Status: TopicStatus{
				AddressStatus: v1beta1.AddressStatus{
					Address: &v1beta1.Addressable{},
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
				// DeepCopy because we will edit it below.
				in := test.in.DeepCopy()
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Topic{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
				// IdentityStatus.ServiceAccountName only exists in v1alpha1 and v1beta1, it doesn't exist in v1.
				// So this won't be a round trip, it will be silently removed.
				in.Status.ServiceAccountName = ""
				if diff := cmp.Diff(in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
