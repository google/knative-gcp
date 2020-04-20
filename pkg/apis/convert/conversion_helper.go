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

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	pkgduckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func ToV1beta1PubSubSpec(from duckv1alpha1.PubSubSpec) duckv1beta1.PubSubSpec {
	to := duckv1beta1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = ToV1beta1IdentitySpec(from.IdentitySpec)
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

func ToV1beta1IdentitySpec(from duckv1alpha1.IdentitySpec) duckv1beta1.IdentitySpec {
	to := duckv1beta1.IdentitySpec{}
	to.GoogleServiceAccount = from.GoogleServiceAccount
	return to
}
func FromV1beta1IdentitySpec(from duckv1beta1.IdentitySpec) duckv1alpha1.IdentitySpec {
	to := duckv1alpha1.IdentitySpec{}
	to.GoogleServiceAccount = from.GoogleServiceAccount
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

func ToV1beta1IdentityStatus(from duckv1alpha1.IdentityStatus) duckv1beta1.IdentityStatus {
	to := duckv1beta1.IdentityStatus{}
	to.Status = from.Status
	to.ServiceAccountName = from.ServiceAccountName
	return to
}
func FromV1beta1IdentityStatus(from duckv1beta1.IdentityStatus) duckv1alpha1.IdentityStatus {
	to := duckv1alpha1.IdentityStatus{}
	to.Status = from.Status
	to.ServiceAccountName = from.ServiceAccountName
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
