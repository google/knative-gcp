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

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/pkg/apis"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
)

// These variables are used to create a 'complete' version of Channel where every field is filled
// in.
var (
	trueVal       = true
	seconds       = int64(314)
	three         = int32(3)
	backoffPolicy = eventingduckv1beta1.BackoffPolicyExponential
	backoffDelay  = "backoffDelay"

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

	completeDestination = pkgduckv1.Destination{
		Ref: &pkgduckv1.KReference{
			APIVersion: "apiVersion",
			Kind:       "kind",
			Namespace:  "namespace",
			Name:       "name",
		},
		URI: &completeURL,
	}

	completeIdentitySpec = duckv1alpha1.IdentitySpec{
		ServiceAccountName: "k8sServiceAccount",
	}

	completeSecret = &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{
			Name: "name",
		},
		Key:      "key",
		Optional: &trueVal,
	}

	completeSubscribable = &eventingduckv1alpha1.Subscribable{
		Subscribers: []eventingduckv1alpha1.SubscriberSpec{
			{
				UID:               "uid-1",
				Generation:        1,
				SubscriberURI:     &completeURL,
				ReplyURI:          &completeURL,
				DeadLetterSinkURI: &completeURL,
				Delivery: &eventingduckv1beta1.DeliverySpec{
					DeadLetterSink: &completeDestination,
					Retry:          &three,
					BackoffPolicy:  &backoffPolicy,
					BackoffDelay:   &backoffDelay,
				},
			},
		},
	}

	completeIdentityStatus = duckv1alpha1.IdentityStatus{
		Status: pkgduckv1.Status{
			ObservedGeneration: 7,
			Conditions: pkgduckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	completeAddressStatus = pkgduckv1.AddressStatus{
		Address: &pkgduckv1.Addressable{
			URL: &completeURL,
		},
	}

	completeSubscribableTypeStatus = eventingduckv1alpha1.SubscribableTypeStatus{
		SubscribableStatus: &eventingduckv1alpha1.SubscribableStatus{
			Subscribers: []eventingduckv1beta1.SubscriberStatus{
				{
					UID:                "uid-1",
					ObservedGeneration: 1,
					Ready:              "ready-1",
					Message:            "message-1",
				},
				{
					UID:                "uid-2",
					ObservedGeneration: 2,
					Ready:              "ready-2",
					Message:            "message-2",
				},
			},
		},
	}

	// completePullSubscription is a Channel with every field filled in, except TypeMeta. TypeMeta
	// is excluded because conversions do not convert it and this variable was created to test
	// conversions.
	completeChannel = &Channel{
		ObjectMeta: completeObjectMeta,
		Spec: ChannelSpec{
			IdentitySpec: completeIdentitySpec,
			Secret:       completeSecret,
			Project:      "project",
			Subscribable: completeSubscribable,
		},
		Status: ChannelStatus{
			IdentityStatus:         completeIdentityStatus,
			AddressStatus:          completeAddressStatus,
			SubscribableTypeStatus: completeSubscribableTypeStatus,
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
				if test.in.Annotations == nil {
					test.in.Annotations = make(map[string]string, 1)
				}
				test.in.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1alpha1"

				// DeadLetterSinkURI exists exclusively in v1alpha1, it has not yet been promoted to
				// v1beta1. So it won't round trip, it will be silently removed.
				if test.in.Spec.Subscribable != nil {
					for i := range test.in.Spec.Subscribable.Subscribers {
						test.in.Spec.Subscribable.Subscribers[i].DeadLetterSinkURI = nil
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
