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

package convert_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/convert"
	gcptesting "github.com/google/knative-gcp/pkg/testing"
)

func TestV1beta1PubSubSpec(t *testing.T) {
	want := gcptesting.CompleteV1alpha1PubSubSpec
	got := convert.FromV1beta1PubSubSpec(convert.ToV1beta1PubSubSpec(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1alpha1ToV1PubSubSpec(t *testing.T) {
	want := gcptesting.CompleteV1alpha1PubSubSpec
	got := convert.FromV1ToV1alpha1PubSubSpec(convert.FromV1alpha1ToV1PubSubSpec(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1PubSubSpec(t *testing.T) {
	want := gcptesting.CompleteV1beta1PubSubSpec
	got := convert.FromV1PubSubSpec(convert.ToV1PubSubSpec(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1IdentitySpec(t *testing.T) {
	want := gcptesting.CompleteV1alpha1IdentitySpec
	got := convert.FromV1beta1IdentitySpec(convert.ToV1beta1IdentitySpec(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1IdentitySpec(t *testing.T) {
	want := gcptesting.CompleteV1beta1IdentitySpec
	got := convert.FromV1IdentitySpec(convert.ToV1IdentitySpec(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1PubSubStatus(t *testing.T) {
	want := gcptesting.CompleteV1alpha1PubSubStatus
	got := convert.FromV1beta1PubSubStatus(convert.ToV1beta1PubSubStatus(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1alpha1toV1PubSubStatus(t *testing.T) {
	want := gcptesting.CompleteV1alpha1PubSubStatusWithoutServiceAccountName
	got := convert.FromV1ToV1alpha1PubSubStatus(convert.FromV1alpha1ToV1PubSubStatus(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1PubSubStatus(t *testing.T) {
	want := gcptesting.CompleteV1beta1PubSubStatus
	got := convert.FromV1PubSubStatus(convert.ToV1PubSubStatus(want))

	// ServiceAccountName exists exclusively in v1alpha1 and v1beta1, it has not yet been promoted to
	// v1. So it won't round trip, it will be silently removed.
	want.ServiceAccountName = ""

	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1IdentityStatus(t *testing.T) {
	want := gcptesting.CompleteV1alpha1IdentityStatus
	got := convert.FromV1beta1IdentityStatus(convert.ToV1beta1IdentityStatus(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1IdentityStatus(t *testing.T) {
	want := gcptesting.CompleteV1beta1IdentityStatus
	got := convert.FromV1IdentityStatus(convert.ToV1IdentityStatus(want))
	// ServiceAccountName exists exclusively in v1alpha1 and v1beta1, it has not yet been promoted to
	// v1. So it won't round trip, it will be silently removed.
	want.ServiceAccountName = ""
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1AddressStatus(t *testing.T) {
	want := gcptesting.CompleteV1alpha1AddressStatus
	v1b1, err := convert.ToV1beta1AddressStatus(context.Background(), want)
	if err != nil {
		t.Fatalf("Unable to convert to v1beta1 %v", err)
	}
	got, err := convert.FromV1beta1AddressStatus(context.Background(), v1b1)
	if err != nil {
		t.Fatalf("Unable to convert from v1beta1 %v", err)
	}

	// Hostname exists in v1alpha1, but not in v1beta1, so it will not be present after
	// the round-trip conversion. Compare it manually, then overwrite the output value
	// so that the generic comparison later works.
	if want.Address != nil {
		if wantHostname := want.Address.Hostname; wantHostname != "" {
			if gotHostname := got.Address.URL.Host; wantHostname != gotHostname {
				// Note that this assumes that if both a URL and a hostname are
				// specified in the original, then they have the same value.
				t.Errorf("Incorrect hostname, want %q, got %q", wantHostname, gotHostname)
			}
			got.Address = want.Address
		}
	}

	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1AddressStatus(t *testing.T) {
	want := gcptesting.CompleteV1beta1AddressStatus
	v1, err := convert.ToV1AddressStatus(context.Background(), want)
	if err != nil {
		t.Fatalf("Unable to convert to v1beta1 %v", err)
	}
	got, err := convert.FromV1AddressStatus(context.Background(), v1)
	if err != nil {
		t.Fatalf("Unable to convert from v1beta1 %v", err)
	}
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1SubscribableSpec(t *testing.T) {
	// DeepCopy because we will edit it below.
	want := gcptesting.CompleteV1alpha1Subscribable.DeepCopy()
	v1b1 := convert.ToV1beta1SubscribableSpec(want)
	got := convert.FromV1beta1SubscribableSpec(v1b1)

	// DeadLetterSinkURI exists exclusively in v1alpha1, it has not yet been promoted to
	// v1beta1. So it won't round trip, it will be silently removed.
	for i := range want.Subscribers {
		want.Subscribers[i].DeadLetterSinkURI = nil
	}

	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1alpha1Deprecated(t *testing.T) {
	cs := apis.NewLivingConditionSet()
	status := duckv1.Status{}
	convert.MarkV1alpha1Deprecated(&cs, &status)
	dc := status.GetCondition("Deprecated")
	if dc == nil {
		t.Errorf("MarkV1alpha1Deprecated should add a deprecated warning condition but it does not.")
	} else if diff := cmp.Diff(*dc, convert.DeprecatedV1Alpha1Condition, cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
		t.Errorf("Failed to verify deprecated condition (-want, + got) = %v", diff)
	}
	convert.RemoveV1alpha1Deprecated(&cs, &status)
	dc = status.GetCondition("Deprecated")
	if dc != nil {
		t.Errorf("RemoveV1alpha1Deprecated should remove the deprecated warning condition but it does not.")
	}
}
