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

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/convert"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var (
	trueVal = true

	completeURL = apis.URL{
		Scheme:     "scheme",
		Opaque:     "opaque",
		User:       url.User("user"),
		Host:       "host",
		Path:       "path",
		RawPath:    "rawPath",
		ForceQuery: false,
		RawQuery:   "rawQuery",
		Fragment:   "fragment",
	}

	completeDestination = duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "apiVersion",
			Kind:       "kind",
			Namespace:  "namespace",
			Name:       "name",
		},
		URI: &completeURL,
	}

	completeSourceSpec = duckv1.SourceSpec{
		Sink: completeDestination,
		CloudEventOverrides: &duckv1.CloudEventOverrides{
			Extensions: map[string]string{"supers": "reckoners"},
		},
	}

	completeIdentitySpec = duckv1alpha1.IdentitySpec{
		GoogleServiceAccount: "googleServiceAccount",
	}

	completeSecret = &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{
			Name: "name",
		},
		Key:      "key",
		Optional: &trueVal,
	}

	completePubSubSpec = duckv1alpha1.PubSubSpec{
		SourceSpec:   completeSourceSpec,
		IdentitySpec: completeIdentitySpec,
		Secret:       completeSecret,
		Project:      "project",
	}

	completeIdentityStatus = duckv1alpha1.IdentityStatus{
		Status: duckv1.Status{
			ObservedGeneration: 7,
			Conditions: duckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		ServiceAccountName: "serviceAccountName",
	}

	completePubSubStatus = duckv1alpha1.PubSubStatus{
		IdentityStatus: completeIdentityStatus,
		SinkURI:        &completeURL,
		CloudEventAttributes: []duckv1.CloudEventAttributes{
			{
				Type:   "type",
				Source: "source",
			},
		},
		ProjectID:      "projectID",
		TopicID:        "topicID",
		SubscriptionID: "subscriptionID",
	}

	completeAddressStatus = pkgduckv1alpha1.AddressStatus{
		Address: &pkgduckv1alpha1.Addressable{
			Addressable: duckv1beta1.Addressable{
				URL: &completeURL,
			},
			Hostname: completeURL.Host,
		},
	}
)

func TestV1beta1PubSubSpec(t *testing.T) {
	want := completePubSubSpec
	got := convert.FromV1beta1PubSubSpec(convert.ToV1beta1PubSubSpec(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1IdentitySpec(t *testing.T) {
	want := completeIdentitySpec
	got := convert.FromV1beta1IdentitySpec(convert.ToV1beta1IdentitySpec(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1PubSubStatus(t *testing.T) {
	want := completePubSubStatus
	got := convert.FromV1beta1PubSubStatus(convert.ToV1beta1PubSubStatus(want))
	ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
	if diff := cmp.Diff(want, got, ignoreUsername); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1IdentityStatus(t *testing.T) {
	want := completeIdentityStatus
	got := convert.FromV1beta1IdentityStatus(convert.ToV1beta1IdentityStatus(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want +got): %v", diff)
	}
}

func TestV1beta1AddressStatus(t *testing.T) {
	want := completeAddressStatus
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
