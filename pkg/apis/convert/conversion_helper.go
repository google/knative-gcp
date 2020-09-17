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

package convert

import (
	"context"

	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	corev1 "k8s.io/api/core/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	pkgduckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

const (
	DeprecatedType           = "Deprecated"
	deprecatedV1Alpha1Reason = "v1alpha1Deprecated"
	deprecatedV1Alpha1Msg    = "V1alpha1 has been deprecated and will be removed in 0.19."
)

var (
	DeprecatedV1Alpha1Condition = apis.Condition{
		Type:     DeprecatedType,
		Reason:   deprecatedV1Alpha1Reason,
		Status:   corev1.ConditionTrue,
		Severity: apis.ConditionSeverityWarning,
		Message:  deprecatedV1Alpha1Msg,
	}
)

func ToV1beta1PubSubSpec(from duckv1alpha1.PubSubSpec) duckv1beta1.PubSubSpec {
	to := duckv1beta1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = ToV1beta1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func FromV1alpha1ToV1PubSubSpec(from duckv1alpha1.PubSubSpec) duckv1.PubSubSpec {
	to := duckv1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = FromV1alpha1ToV1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func ToV1PubSubSpec(from duckv1beta1.PubSubSpec) duckv1.PubSubSpec {
	to := duckv1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = ToV1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func FromV1beta1PubSubSpec(from duckv1beta1.PubSubSpec) duckv1alpha1.PubSubSpec {
	to := duckv1alpha1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = FromV1beta1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func FromV1ToV1alpha1PubSubSpec(from duckv1.PubSubSpec) duckv1alpha1.PubSubSpec {
	to := duckv1alpha1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = FromV1ToV1alpha1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func FromV1PubSubSpec(from duckv1.PubSubSpec) duckv1beta1.PubSubSpec {
	to := duckv1beta1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = FromV1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func ToV1beta1IdentitySpec(from duckv1alpha1.IdentitySpec) duckv1beta1.IdentitySpec {
	to := duckv1beta1.IdentitySpec{}
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func FromV1alpha1ToV1IdentitySpec(from duckv1alpha1.IdentitySpec) duckv1.IdentitySpec {
	to := duckv1.IdentitySpec{}
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func ToV1IdentitySpec(from duckv1beta1.IdentitySpec) duckv1.IdentitySpec {
	to := duckv1.IdentitySpec{}
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func FromV1beta1IdentitySpec(from duckv1beta1.IdentitySpec) duckv1alpha1.IdentitySpec {
	to := duckv1alpha1.IdentitySpec{}
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func FromV1ToV1alpha1IdentitySpec(from duckv1.IdentitySpec) duckv1alpha1.IdentitySpec {
	to := duckv1alpha1.IdentitySpec{}
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func FromV1IdentitySpec(from duckv1.IdentitySpec) duckv1beta1.IdentitySpec {
	to := duckv1beta1.IdentitySpec{}
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func ToV1beta1PubSubStatus(from duckv1alpha1.PubSubStatus) duckv1beta1.PubSubStatus {
	to := duckv1beta1.PubSubStatus{}
	to.IdentityStatus = ToV1beta1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func FromV1alpha1ToV1PubSubStatus(from duckv1alpha1.PubSubStatus) duckv1.PubSubStatus {
	to := duckv1.PubSubStatus{}
	to.IdentityStatus = FromV1alpha1ToV1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func ToV1PubSubStatus(from duckv1beta1.PubSubStatus) duckv1.PubSubStatus {
	to := duckv1.PubSubStatus{}
	to.IdentityStatus = ToV1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func FromV1beta1PubSubStatus(from duckv1beta1.PubSubStatus) duckv1alpha1.PubSubStatus {
	to := duckv1alpha1.PubSubStatus{}
	to.IdentityStatus = FromV1beta1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func FromV1ToV1alpha1PubSubStatus(from duckv1.PubSubStatus) duckv1alpha1.PubSubStatus {
	to := duckv1alpha1.PubSubStatus{}
	to.IdentityStatus = FromV1ToV1alpha1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func FromV1PubSubStatus(from duckv1.PubSubStatus) duckv1beta1.PubSubStatus {
	to := duckv1beta1.PubSubStatus{}
	to.IdentityStatus = FromV1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func ToV1beta1IdentityStatus(from duckv1alpha1.IdentityStatus) duckv1beta1.IdentityStatus {
	to := duckv1beta1.IdentityStatus{}
	to.Status = from.Status
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func FromV1alpha1ToV1IdentityStatus(from duckv1alpha1.IdentityStatus) duckv1.IdentityStatus {
	to := duckv1.IdentityStatus{}
	to.Status = from.Status
	return to
}

func ToV1IdentityStatus(from duckv1beta1.IdentityStatus) duckv1.IdentityStatus {
	to := duckv1.IdentityStatus{}
	to.Status = from.Status
	return to
}

func FromV1beta1IdentityStatus(from duckv1beta1.IdentityStatus) duckv1alpha1.IdentityStatus {
	to := duckv1alpha1.IdentityStatus{}
	to.Status = from.Status
	to.ServiceAccountName = from.ServiceAccountName
	return to
}

func FromV1ToV1alpha1IdentityStatus(from duckv1.IdentityStatus) duckv1alpha1.IdentityStatus {
	to := duckv1alpha1.IdentityStatus{}
	to.Status = from.Status
	return to
}

func FromV1IdentityStatus(from duckv1.IdentityStatus) duckv1beta1.IdentityStatus {
	to := duckv1beta1.IdentityStatus{}
	to.Status = from.Status
	return to
}

func ToV1beta1AddressStatus(ctx context.Context, from pkgduckv1alpha1.AddressStatus) (pkgduckv1beta1.AddressStatus, error) {
	to := pkgduckv1beta1.AddressStatus{}
	if from.Address != nil {
		to.Address = &pkgduckv1beta1.Addressable{}
		err := from.Address.ConvertTo(ctx, to.Address)
		if err != nil {
			return pkgduckv1beta1.AddressStatus{}, err
		}
	}
	return to, nil
}

func ToV1AddressStatus(ctx context.Context, from pkgduckv1beta1.AddressStatus) (pkgduckv1.AddressStatus, error) {
	to := pkgduckv1.AddressStatus{}
	if from.Address != nil {
		to.Address = &pkgduckv1.Addressable{}
		err := from.Address.ConvertTo(ctx, to.Address)
		if err != nil {
			return pkgduckv1.AddressStatus{}, err
		}
	}
	return to, nil
}

func FromV1beta1AddressStatus(ctx context.Context, from pkgduckv1beta1.AddressStatus) (pkgduckv1alpha1.AddressStatus, error) {
	to := pkgduckv1alpha1.AddressStatus{}
	if from.Address != nil {
		to.Address = &pkgduckv1alpha1.Addressable{}
		err := to.Address.ConvertFrom(ctx, from.Address)
		if err != nil {
			return pkgduckv1alpha1.AddressStatus{}, err
		}
	}
	return to, nil
}

func FromV1AddressStatus(ctx context.Context, from pkgduckv1.AddressStatus) (pkgduckv1beta1.AddressStatus, error) {
	to := pkgduckv1beta1.AddressStatus{}
	if from.Address != nil {
		to.Address = &pkgduckv1beta1.Addressable{}
		err := to.Address.ConvertFrom(ctx, from.Address)
		if err != nil {
			return pkgduckv1beta1.AddressStatus{}, err
		}
	}
	return to, nil
}

func ToV1beta1SubscribableSpec(from *eventingduckv1alpha1.Subscribable) *eventingduckv1beta1.SubscribableSpec {
	if from == nil {
		return nil
	}
	to := eventingduckv1beta1.SubscribableSpec{}
	to.Subscribers = make([]eventingduckv1beta1.SubscriberSpec, len(from.Subscribers))
	for i, sub := range from.Subscribers {
		to.Subscribers[i] = eventingduckv1beta1.SubscriberSpec{
			UID:           sub.UID,
			Generation:    sub.Generation,
			SubscriberURI: sub.SubscriberURI,
			ReplyURI:      sub.ReplyURI,
			// DeadLetterSinkURI doesn't exist in v1beta1, so don't translate it.
			Delivery: sub.Delivery,
		}
	}
	return &to
}

func FromV1beta1SubscribableSpec(from *eventingduckv1beta1.SubscribableSpec) *eventingduckv1alpha1.Subscribable {
	if from == nil {
		return nil
	}
	to := eventingduckv1alpha1.Subscribable{}
	to.Subscribers = make([]eventingduckv1alpha1.SubscriberSpec, len(from.Subscribers))
	for i, sub := range from.Subscribers {
		to.Subscribers[i] = eventingduckv1alpha1.SubscriberSpec{
			UID:           sub.UID,
			Generation:    sub.Generation,
			SubscriberURI: sub.SubscriberURI,
			ReplyURI:      sub.ReplyURI,
			// DeadLetterSinkURI doesn't exist in v1beta1, so don't translate it.
			Delivery: sub.Delivery,
		}
	}
	return &to
}

// A helper function to mark v1alpha1 Deprecated Condition in the Status.
// We mark the Condition in status during conversion from a higher version to v1alpha1.
// TODO(https://github.com/google/knative-gcp/issues/1544): remove after the 0.18 cut.
func MarkV1alpha1Deprecated(cs *apis.ConditionSet, s *pkgduckv1.Status) {
	cs.Manage(s).SetCondition(DeprecatedV1Alpha1Condition)
}

// A helper function to remove Deprecated Condition from the Status during conversion
// from v1alpha1 to a higher version.
// TODO(https://github.com/google/knative-gcp/issues/1544): remove after the 0.18 cut.
func RemoveV1alpha1Deprecated(cs *apis.ConditionSet, s *pkgduckv1.Status) {
	cs.Manage(s).ClearCondition(DeprecatedType)
}
