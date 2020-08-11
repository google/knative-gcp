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

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/convert"

	"knative.dev/pkg/apis/duck/v1alpha1"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	gcptesting "github.com/google/knative-gcp/pkg/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// These variables are used to create a 'complete' version of Topic where every field is filled in.
var (
	// completeTopic is a Topic with every filled in, except TypeMeta. TypeMeta is excluded because
	// conversions do not convert it and this variable was created to test conversions.
	completeTopic = &Topic{
		ObjectMeta: gcptesting.CompleteObjectMeta,
		Spec: TopicSpec{
			IdentitySpec:      gcptesting.CompleteV1alpha1IdentitySpec,
			Secret:            gcptesting.CompleteSecret,
			Project:           "project",
			Topic:             "topic",
			PropagationPolicy: TopicPolicyCreateDelete,
			EnablePublisher:   &trueVal,
		},
		Status: TopicStatus{
			IdentityStatus: gcptesting.CompleteV1alpha1IdentityStatus,
			AddressStatus:  gcptesting.CompleteV1alpha1AddressStatus,
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

func TestTopicConversionBetweenV1beta1(t *testing.T) {
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
			ObjectMeta: gcptesting.CompleteObjectMeta,
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

				// Make sure the Deprecated Condition is added to the Status after converted back to v1alpha1,
				// We need to ignore the LastTransitionTime as it is set in real time when doing the comparison.
				dc := got.Status.GetCondition(convert.DeprecatedType)
				if dc == nil {
					t.Errorf("ConvertFrom() should add a deprecated warning condition but it does not.")
				} else if diff := cmp.Diff(*dc, convert.DeprecatedV1Alpha1Condition,
					cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
					t.Errorf("Failed to verify deprecated condition (-want, + got) = %v", diff)
				}
				// Remove the Deprecated Condition from Status to compare the remaining of fields.
				cs := apis.NewLivingConditionSet()
				cs.Manage(&got.Status).ClearCondition(convert.DeprecatedType)

				ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
				if diff := cmp.Diff(test.in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

func TestTopicConversionBetweenV1(t *testing.T) {
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
				// DeepCopy because we will edit it below.
				in := test.in.DeepCopy()
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
				// Make sure the Deprecated Condition is added to the Status after converted back to v1alpha1,
				// We need to ignore the LastTransitionTime as it is set in real time when doing the comparison.
				dc := got.Status.GetCondition(convert.DeprecatedType)
				if dc == nil {
					t.Errorf("ConvertFrom() should add a deprecated warning condition but it does not.")
				} else if diff := cmp.Diff(*dc, convert.DeprecatedV1Alpha1Condition,
					cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
					t.Errorf("Failed to verify deprecated condition (-want, + got) = %v", diff)
				}
				// Remove the Deprecated Condition from Status to compare the remaining of fields.
				cs := apis.NewLivingConditionSet()
				cs.Manage(&got.Status).ClearCondition(convert.DeprecatedType)

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
