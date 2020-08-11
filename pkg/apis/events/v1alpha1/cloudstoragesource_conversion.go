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

	"github.com/google/knative-gcp/pkg/apis/convert"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts from v1alpha1.CloudStorageSource into a higher version.
func (source *CloudStorageSource) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.CloudStorageSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.ToV1beta1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.ServiceAccountName = source.Spec.ServiceAccountName
		sink.Spec.Bucket = source.Spec.Bucket
		sink.Spec.EventTypes = source.Spec.EventTypes
		sink.Spec.ObjectNamePrefix = source.Spec.ObjectNamePrefix
		sink.Spec.PayloadFormat = source.Spec.PayloadFormat
		sink.Status.PubSubStatus = convert.ToV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.NotificationID = source.Status.NotificationID
		// Remove v1alpha1 as deprecated from the Status Condition when converting to a higher version.
		convert.RemoveV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta1.CloudStorageSource{}, sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts from a higher version into v1alpha1.CloudStorageSource.
func (sink *CloudStorageSource) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.CloudStorageSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.FromV1beta1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.ServiceAccountName = source.Spec.ServiceAccountName
		sink.Spec.Bucket = source.Spec.Bucket
		sink.Spec.EventTypes = source.Spec.EventTypes
		sink.Spec.ObjectNamePrefix = source.Spec.ObjectNamePrefix
		sink.Spec.PayloadFormat = source.Spec.PayloadFormat
		sink.Status.PubSubStatus = convert.FromV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.NotificationID = source.Status.NotificationID
		// Mark v1alpha1 as deprecated as a Status Condition when converting to v1alpha1.
		convert.MarkV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta1.CloudStorageSource{}, sink)
	}
}
