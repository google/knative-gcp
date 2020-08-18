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

	convert "github.com/google/knative-gcp/pkg/apis/convert"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts a v1alpha1.PullSubscription to a higher version of PullSubscription.
func (source *PullSubscription) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.PullSubscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.ToV1beta1PubSubSpec(source.Spec.PubSubSpec)
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
		sink.Status.PubSubStatus = convert.ToV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.TransformerURI = source.Status.TransformerURI
		sink.Status.SubscriptionID = source.Status.SubscriptionID
		// Remove v1alpha1 as deprecated from the Status Condition when converting to a higher version.
		convert.RemoveV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta1.PullSubscription{}, sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts from a higher version of PullSubscription to a v1alpha1.PullSubscription.
func (sink *PullSubscription) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.PullSubscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.FromV1beta1PubSubSpec(source.Spec.PubSubSpec)
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
		sink.Status.PubSubStatus = convert.FromV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.TransformerURI = source.Status.TransformerURI
		sink.Status.SubscriptionID = source.Status.SubscriptionID
		// Mark v1alpha1 as deprecated as a Status Condition when converting to v1alpha1.
		convert.MarkV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta1.PullSubscription{}, sink)
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
