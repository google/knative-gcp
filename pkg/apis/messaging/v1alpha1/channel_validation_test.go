/*
Copyright 2019 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var (
	validServiceAccountName   = "test123@test123.iam.gserviceaccount.com"
	invalidServiceAccountName = "test@test.iam.kserviceaccount.com"

	channelSpec = ChannelSpec{
		Subscribable: &eventingduck.Subscribable{
			Subscribers: []eventingduck.SubscriberSpec{{
				SubscriberURI: apis.HTTP("subscriberendpoint"),
				ReplyURI:      apis.HTTP("resultendpoint"),
			}},
		},
	}
)

func TestChannelValidation(t *testing.T) {
	tests := []struct {
		name string
		cr   resourcesemantics.GenericCRD
		want *apis.FieldError
	}{{
		name: "empty",
		cr: &Channel{
			Spec: ChannelSpec{},
		},
		want: nil,
	}, {
		name: "valid subscribers array",
		cr: &Channel{
			Spec: ChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("resultendpoint"),
					}},
				}},
		},
		want: nil,
	}, {
		name: "empty subscriber at index 1",
		cr: &Channel{
			Spec: ChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}, {}},
				}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			return fe
		}(),
	}, {
		name: "2 empty subscribers",
		cr: &Channel{
			Spec: ChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{}, {}},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("spec.subscribable.subscriber[0].replyURI", "spec.subscribable.subscriber[0].subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe)
			fe = apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
			fe.Details = "expected at least one of, got none"
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "nil secret",
		cr: &Channel{
			Spec: ChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}},
				}},
		},
		want: nil,
	}, {
		name: "invalid GCP service account",
		cr: &Channel{
			Spec: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					GoogleServiceAccount: invalidServiceAccountName,
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}},
				}},
		},
		want: func() *apis.FieldError {
			fe := &apis.FieldError{
				Message: `invalid value: test@test.iam.kserviceaccount.com, serviceAccount should have format: ^[a-z][a-z0-9-]{5,29}@[a-z][a-z0-9-]{5,29}.iam.gserviceaccount.com$`,
				Paths:   []string{"spec.serviceAccount"},
			}
			return fe
		}(),
	}, {
		name: "valid GCP service account",
		cr: &Channel{
			Spec: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					GoogleServiceAccount: validServiceAccountName,
				},
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}},
				}},
		},
		want: nil,
	}, {
		name: "have GCP service account and secret at the same time",
		cr: &Channel{
			Spec: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					GoogleServiceAccount: validServiceAccountName,
				},
				Secret: defaultSecretSelector(),
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}},
				}},
		},
		want: func() *apis.FieldError {
			fe := &apis.FieldError{
				Message: "Can't have spec.serviceAccount and spec.secret at the same time",
				Paths:   []string{"spec"},
			}
			return fe
		}(),
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cr.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validate (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    interface{}
		updated ChannelSpec
		allowed bool
	}{
		"nil orig": {
			updated: ChannelSpec{},
			allowed: true,
		},
		"ServiceAccount changed": {
			orig: &channelSpec,
			updated: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					GoogleServiceAccount: "new-service-account",
				},
			},
			allowed: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *Channel

			if tc.orig != nil {
				if spec, ok := tc.orig.(*ChannelSpec); ok {
					orig = &Channel{
						Spec: *spec,
					}
				}
			}
			updated := &Channel{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
