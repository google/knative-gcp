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

	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// These variables are used to create a 'complete' version of CloudStorageSource where every field is
// filled in.
var (
	// completeCloudStorageSource is a CloudStorageSource with every field filled in, except TypeMeta.
	// TypeMeta is excluded because conversions do not convert it and this variable was created to
	// test conversions.
	completeCloudStorageSource = &CloudStorageSource{
		ObjectMeta: completeObjectMeta,
		Spec: CloudStorageSourceSpec{
			PubSubSpec:       completePubSubSpec,
			Bucket:           "bucket",
			EventTypes:       []string{"event", "types"},
			ObjectNamePrefix: "objectNamePrefix",
			PayloadFormat:    "payloadFormat",
		},
		Status: CloudStorageSourceStatus{
			PubSubStatus:   completePubSubStatus,
			NotificationID: "notificationId",
		},
	}
)

func TestCloudStorageSourceConversionBadType(t *testing.T) {
	good, bad := &CloudStorageSource{}, &CloudStorageSource{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestCloudStorageSourceConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.CloudStorageSource{}}

	tests := []struct {
		name string
		in   *CloudStorageSource
	}{{
		name: "min configuration",
		in: &CloudStorageSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ps-name",
				Namespace:  "ps-ns",
				Generation: 17,
			},
			Spec: CloudStorageSourceSpec{},
		},
	}, {
		name: "full configuration",
		in:   completeCloudStorageSource,
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &CloudStorageSource{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
				if diff := cmp.Diff(test.in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
