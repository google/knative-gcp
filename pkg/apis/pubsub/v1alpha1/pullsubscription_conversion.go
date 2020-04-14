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
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1beta1.PullSubscription) into v1alpha1.PullSubscription.
func (source *PullSubscription) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.PullSubscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convertToV1beta1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.Topic = source.Spec.Topic
		sink.Spec.AckDeadline = source.Spec.AckDeadline
		sink.Spec.RetainAckedMessages = source.Spec.RetainAckedMessages
		sink.Spec.RetentionDuration = source.Spec.RetentionDuration
		sink.Spec.Transformer = source.Spec.Transformer
		if mode, err := convertToV1beta1ModeType(source.Spec.Mode); err != nil {
			return err
		} else {
			sink.Spec.Mode = mode
		}
		sink.Spec.AdapterType = source.Spec.AdapterType
		sink.Status.PubSubStatus = convertToV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.TransformerURI = source.Status.TransformerURI
		sink.Status.SubscriptionID = source.Status.SubscriptionID
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha1.PullSubscription into v1beta1.PullSubscription.
func (sink *PullSubscription) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.PullSubscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convertFromV1beta1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.Topic = source.Spec.Topic
		sink.Spec.AckDeadline = source.Spec.AckDeadline
		sink.Spec.RetainAckedMessages = source.Spec.RetainAckedMessages
		sink.Spec.RetentionDuration = source.Spec.RetentionDuration
		sink.Spec.Transformer = source.Spec.Transformer
		if mode, err := convertFromV1beta1ModeType(source.Spec.Mode); err != nil {
			return err
		} else {
			sink.Spec.Mode = mode
		}
		sink.Spec.AdapterType = source.Spec.AdapterType
		sink.Status.PubSubStatus = convertFromV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.TransformerURI = source.Status.TransformerURI
		sink.Status.SubscriptionID = source.Status.SubscriptionID
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", source)
	}
}

func convertToV1beta1ModeType(from ModeType) (v1beta1.ModeType, error) {
	switch from {
	case ModeCloudEventsBinary:
		return v1beta1.ModeCloudEventsBinary, nil
	case ModeCloudEventsStructured:
		return v1beta1.ModeCloudEventsStructured, nil
	case ModePushCompatible:
		return v1beta1.ModePushCompatible, nil
	case "":
		return "", nil
	default:
		return "unknown", fmt.Errorf("unknown ModeType %v", from)
	}
}

func convertFromV1beta1ModeType(from v1beta1.ModeType) (ModeType, error) {
	switch from {
	case v1beta1.ModeCloudEventsBinary:
		return ModeCloudEventsBinary, nil
	case v1beta1.ModeCloudEventsStructured:
		return ModeCloudEventsStructured, nil
	case v1beta1.ModePushCompatible:
		return ModePushCompatible, nil
	case "":
		return "", nil
	default:
		return "unknown", fmt.Errorf("unknown ModeType %v", from)
	}
}

func convertToV1beta1PubSubSpec(from v1alpha1.PubSubSpec) duckv1beta1.PubSubSpec {
	to := duckv1beta1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = convertToV1beta1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}
func convertFromV1beta1PubSubSpec(from duckv1beta1.PubSubSpec) v1alpha1.PubSubSpec {
	to := v1alpha1.PubSubSpec{}
	to.SourceSpec = from.SourceSpec
	to.IdentitySpec = convertFromV1beta1IdentitySpec(from.IdentitySpec)
	to.Secret = from.Secret
	to.Project = from.Project
	return to
}

func convertToV1beta1IdentitySpec(from v1alpha1.IdentitySpec) duckv1beta1.IdentitySpec {
	to := duckv1beta1.IdentitySpec{}
	to.GoogleServiceAccount = from.GoogleServiceAccount
	return to
}
func convertFromV1beta1IdentitySpec(from duckv1beta1.IdentitySpec) v1alpha1.IdentitySpec {
	to := v1alpha1.IdentitySpec{}
	to.GoogleServiceAccount = from.GoogleServiceAccount
	return to
}

func convertToV1beta1PubSubStatus(from v1alpha1.PubSubStatus) duckv1beta1.PubSubStatus {
	to := duckv1beta1.PubSubStatus{}
	to.IdentityStatus = convertToV1beta1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}
func convertFromV1beta1PubSubStatus(from duckv1beta1.PubSubStatus) v1alpha1.PubSubStatus {
	to := v1alpha1.PubSubStatus{}
	to.IdentityStatus = convertFromV1beta1IdentityStatus(from.IdentityStatus)
	to.SinkURI = from.SinkURI
	to.CloudEventAttributes = from.CloudEventAttributes
	to.ProjectID = from.ProjectID
	to.TopicID = from.TopicID
	to.SubscriptionID = from.SubscriptionID
	return to
}

func convertToV1beta1IdentityStatus(from v1alpha1.IdentityStatus) duckv1beta1.IdentityStatus {
	to := duckv1beta1.IdentityStatus{}
	to.Status = from.Status
	to.ServiceAccountName = from.ServiceAccountName
	return to
}
func convertFromV1beta1IdentityStatus(from duckv1beta1.IdentityStatus) v1alpha1.IdentityStatus {
	to := v1alpha1.IdentityStatus{}
	to.Status = from.Status
	to.ServiceAccountName = from.ServiceAccountName
	return to
}
