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
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// These variables are used to create a 'complete' version of PullSubscription where every field is
// filled in.
var (
	trueVal  = true
	seconds  = int64(314)
	duration = "30s"

	completeObjectMeta = metav1.ObjectMeta{
		Name:            "name",
		GenerateName:    "generateName",
		Namespace:       "namespace",
		SelfLink:        "selfLink",
		UID:             "uid",
		ResourceVersion: "resourceVersion",
		Generation:      2012,
		CreationTimestamp: metav1.Time{
			Time: time.Unix(1, 1),
		},
		DeletionTimestamp: &metav1.Time{
			Time: time.Unix(2, 3),
		},
		DeletionGracePeriodSeconds: &seconds,
		Labels:                     map[string]string{"steel": "heart"},
		Annotations:                map[string]string{"New": "Cago"},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         "apiVersion",
				Kind:               "kind",
				Name:               "n",
				UID:                "uid",
				Controller:         &trueVal,
				BlockOwnerDeletion: &trueVal,
			},
		},
		Finalizers:  []string{"finalizer-1", "finalizer-2"},
		ClusterName: "clusterName",
	}

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

	// completePullSubscription is a PullSubscription with every filled in, except TypeMeta.
	// TypeMeta is excluded because conversions do not convert it and this variable was created to
	// test conversions.
	completePullSubscription = &PullSubscription{
		ObjectMeta: completeObjectMeta,
		Spec: PullSubscriptionSpec{
			PubSubSpec:          completePubSubSpec,
			Topic:               "topic",
			AckDeadline:         &duration,
			RetainAckedMessages: false,
			RetentionDuration:   &duration,
			Transformer:         &completeDestination,
			Mode:                ModeCloudEventsBinary,
			AdapterType:         "adapterType",
		},
		Status: PullSubscriptionStatus{
			PubSubStatus:   completePubSubStatus,
			TransformerURI: &completeURL,
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

func TestPullSubscriptionConversion(t *testing.T) {
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
				ignoreUsername := cmp.AllowUnexported(url.Userinfo{})
				if diff := cmp.Diff(test.in, got, ignoreUsername); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
