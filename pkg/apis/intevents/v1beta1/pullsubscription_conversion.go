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

package v1beta1

import (
	"context"

	"github.com/google/knative-gcp/pkg/apis/convert"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts v1beta1.PullSubscription to a higher version of PullSubscription.
// Currently, we only support v1 as a higher version.
func (source *PullSubscription) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1.PullSubscription:
		// Since we remove Mode from PullSubscriptionSpec in v1, we silently remove it here.
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.ToV1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.Topic = source.Spec.Topic
		sink.Spec.AckDeadline = source.Spec.AckDeadline
		sink.Spec.RetainAckedMessages = source.Spec.RetainAckedMessages
		sink.Spec.RetentionDuration = source.Spec.RetentionDuration
		sink.Spec.Transformer = source.Spec.Transformer
		sink.Spec.AdapterType = source.Spec.AdapterType
		sink.Status.PubSubStatus = convert.ToV1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.TransformerURI = source.Status.TransformerURI
		sink.Status.SubscriptionID = source.Status.SubscriptionID
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1.PullSubscription{}, sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from a higher version of PullSubscription to v1beta1.PullSubscription.
// Currently, we only support v1 as a higher version.
func (sink *PullSubscription) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1.PullSubscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.FromV1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.Topic = source.Spec.Topic
		sink.Spec.AckDeadline = source.Spec.AckDeadline
		sink.Spec.RetainAckedMessages = source.Spec.RetainAckedMessages
		sink.Spec.RetentionDuration = source.Spec.RetentionDuration
		sink.Spec.Transformer = source.Spec.Transformer
		// Since we remove Mode from PullSubscriptionSpec in v1, we treat it as an empty string.
		sink.Spec.Mode = ""
		sink.Spec.AdapterType = source.Spec.AdapterType
		sink.Status.PubSubStatus = convert.FromV1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.TransformerURI = source.Status.TransformerURI
		sink.Status.SubscriptionID = source.Status.SubscriptionID
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1.PullSubscription{}, sink)
	}
}
