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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/convert"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	gcptesting "github.com/google/knative-gcp/pkg/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/pkg/apis"
)

// These variables are used to create a 'complete' version of Channel where every field is filled
// in.
var (
	// completePullSubscription is a Channel with every field filled in, except TypeMeta. TypeMeta
	// is excluded because conversions do not convert it and this variable was created to test
	// conversions.
	completeChannel = &Channel{
		ObjectMeta: gcptesting.CompleteObjectMeta,
		Spec: ChannelSpec{
			IdentitySpec: gcptesting.CompleteV1alpha1IdentitySpec,
			Secret:       gcptesting.CompleteSecret,
			Project:      "project",
			Subscribable: gcptesting.CompleteV1alpha1Subscribable,
		},
		Status: ChannelStatus{
			IdentityStatus:         gcptesting.CompleteV1alpha1IdentityStatus,
			AddressStatus:          gcptesting.CompleteV1AddressStatus,
			SubscribableTypeStatus: gcptesting.CompleteV1alpha1SubscribableTypeStatus,
			ProjectID:              "projectId",
			TopicID:                "topicID",
		},
	}
)

func TestChannelConversionBadType(t *testing.T) {
	good, bad := &Channel{}, &v1alpha1.Topic{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestChannelConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []struct {
		version                 apis.Convertible
		duckSubscribableVersion string
	}{
		{
			version:                 &v1beta1.Channel{},
			duckSubscribableVersion: "v1beta1",
		},
	}

	tests := []struct {
		name string
		in   *Channel
	}{{
		name: "min configuration",
		in: &Channel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "c-name",
				Namespace:  "c-ns",
				Generation: 17,
			},
			Spec: ChannelSpec{},
		},
	}, {
		name: "full configuration",
		in:   completeChannel,
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version.version
				// DeepCopy because we will edit it below.
				in := test.in.DeepCopy()
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}

				// Verify that after conversion the duck subscribable annotation matches the version
				// converted to.
				o := ver.(metav1.Object)
				if sv := o.GetAnnotations()[messaging.SubscribableDuckVersionAnnotation]; sv != version.duckSubscribableVersion {
					t.Errorf("Incorrect subscribable duck version annotation. Want %q. Got %q", version.duckSubscribableVersion, sv)
				}

				got := &Channel{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				// The duck subscribable version of a v1alpha1 Channel will always be set to
				// v1alpha1, even if it was not set in the original.
				if in.Annotations == nil {
					in.Annotations = make(map[string]string, 1)
				}
				in.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1alpha1"

				// DeadLetterSinkURI exists exclusively in v1alpha1, it has not yet been promoted to
				// v1beta1. So it won't round trip, it will be silently removed.
				if in.Spec.Subscribable != nil {
					for i := range in.Spec.Subscribable.Subscribers {
						in.Spec.Subscribable.Subscribers[i].DeadLetterSinkURI = nil
					}
				}

				// V1beta1 Channel implements duck v1 identifiable, where ServiceAccountName is removed.
				// So this is not a round trip, the field will be silently removed.
				in.Status.ServiceAccountName = ""
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
				if diff := cmp.Diff(in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
