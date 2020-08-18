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

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"
	"github.com/google/knative-gcp/pkg/apis/duck"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var (
	validServiceAccountName   = "test"
	invalidServiceAccountName = "@test"

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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: ChannelSpec{},
		},
		want: nil,
	}, {
		name: "valid subscribers array",
		cr: &Channel{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
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
		name: "invalid k8s service account",
		cr: &Channel{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					ServiceAccountName: invalidServiceAccountName,
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
				Message: `invalid value: @test, serviceAccountName should have format: ^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`,
				Paths:   []string{"spec.serviceAccountName"},
			}
			return fe
		}(),
	}, {
		name: "valid k8s service account",
		cr: &Channel{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					ServiceAccountName: validServiceAccountName,
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
		name: "have k8s service account and secret at the same time",
		cr: &Channel{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					ServiceAccountName: validServiceAccountName,
				},
				Secret: &gcpauthtesthelper.Secret,
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{{
						SubscriberURI: apis.HTTP("subscriberendpoint"),
						ReplyURI:      apis.HTTP("replyendpoint"),
					}},
				}},
		},
		want: func() *apis.FieldError {
			fe := &apis.FieldError{
				Message: "Can't have spec.serviceAccountName and spec.secret at the same time",
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
		orig              interface{}
		updated           ChannelSpec
		origAnnotation    map[string]string
		updatedAnnotation map[string]string
		allowed           bool
	}{
		"nil orig": {
			updated: ChannelSpec{},
			allowed: true,
		},
		"ServiceAccount changed": {
			orig: &channelSpec,
			updated: ChannelSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
					ServiceAccountName: "new-service-account",
				},
			},
			allowed: false,
		},
		"Secret changed": {
			orig: &channelSpec,
			updated: ChannelSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "some-other-name",
					},
					Key: "some-other-secret-key",
				},
			},
			allowed: false,
		},
		"Project changed": {
			orig: &channelSpec,
			updated: ChannelSpec{
				Project: "some-other-project",
			},
			allowed: false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *Channel

			if tc.origAnnotation != nil {
				orig = &Channel{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: tc.origAnnotation,
					},
				}
			} else if tc.orig != nil {
				if spec, ok := tc.orig.(*ChannelSpec); ok {
					orig = &Channel{
						Spec: *spec,
					}
				}
			}
			updated := &Channel{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.updatedAnnotation,
				},
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
