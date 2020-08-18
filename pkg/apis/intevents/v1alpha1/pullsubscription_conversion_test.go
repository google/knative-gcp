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

	"github.com/google/go-cmp/cmp"
	inteventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	gcptesting "github.com/google/knative-gcp/pkg/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// These variables are used to create a 'complete' version of PullSubscription where every field is
// filled in.
var (
	// completePullSubscription is a PullSubscription with every field filled in, except TypeMeta.
	// TypeMeta is excluded because conversions do not convert it and this variable was created to
	// test conversions.
	completePullSubscription = &PullSubscription{
		ObjectMeta: gcptesting.CompleteObjectMeta,
		Spec: PullSubscriptionSpec{
			PubSubSpec:          gcptesting.CompleteV1alpha1PubSubSpec,
			Topic:               "topic",
			AckDeadline:         &gcptesting.RetentionDuration,
			RetainAckedMessages: false,
			RetentionDuration:   &gcptesting.RetentionDuration,
			Transformer:         &gcptesting.CompleteDestination,
			Mode:                ModeCloudEventsBinary,
			AdapterType:         "adapterType",
		},
		Status: PullSubscriptionStatus{
			PubSubStatus:   gcptesting.CompleteV1alpha1PubSubStatus,
			TransformerURI: &gcptesting.CompleteURL,
			SubscriptionID: "subscriptionID",
		},
	}
)

func TestPullSubscriptionConversionBadType(t *testing.T) {
	good, bad := &PullSubscription{}, &Topic{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestPullSubscriptionConversionBetweenV1Beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.PullSubscription{}}

	tests := []struct {
		name string
		in   *PullSubscription
	}{{
		name: "min configuration",
		in: &PullSubscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ps-name",
				Namespace:  "ps-ns",
				Generation: 17,
			},
			Spec: PullSubscriptionSpec{},
		},
	}, {
		name: "full configuration",
		in:   completePullSubscription,
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &PullSubscription{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
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

func TestPullSubscriptionConversionBetweenV1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&inteventsv1.PullSubscription{}}

	tests := []struct {
		name string
		in   *PullSubscription
	}{{
		name: "min configuration",
		in: &PullSubscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ps-name",
				Namespace:  "ps-ns",
				Generation: 17,
			},
			Spec: PullSubscriptionSpec{},
		},
	}, {
		name: "full configuration",
		in:   completePullSubscription,
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
				got := &PullSubscription{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
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
				// IdentityStatus.ServiceAccountName and PullSubscriptionSpec.Mode only exists in v1alpha1 and v1beta1, they don't exist in v1.
				// So this won't be a round trip, they will be silently removed.
				in.Status.ServiceAccountName = ""
				in.Spec.Mode = ""
				if diff := cmp.Diff(in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
