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
// Converts from a v1alpha1.CloudAuditLogsSource to a higher version of CloudAuditLogsSource.
func (source *CloudAuditLogsSource) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.CloudAuditLogsSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.ToV1beta1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.ServiceName = source.Spec.ServiceName
		sink.Spec.MethodName = source.Spec.MethodName
		sink.Spec.ResourceName = source.Spec.ResourceName
		sink.Status.PubSubStatus = convert.ToV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.StackdriverSink = source.Status.StackdriverSink
		// Remove v1alpha1 as deprecated from the Status Condition when converting to a higher version.
		convert.RemoveV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta1.CloudAuditLogsSource{}, sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts from a higher version of CloudAuditLogsSource to a v1alpha1.CloudAuditLogsSource.
func (sink *CloudAuditLogsSource) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.CloudAuditLogsSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.FromV1beta1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.ServiceName = source.Spec.ServiceName
		sink.Spec.MethodName = source.Spec.MethodName
		sink.Spec.ResourceName = source.Spec.ResourceName
		sink.Status.PubSubStatus = convert.FromV1beta1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.StackdriverSink = source.Status.StackdriverSink
		// Mark v1alpha1 as deprecated as a Status Condition when converting to v1alpha1.
		convert.MarkV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta1.CloudAuditLogsSource{}, sink)
	}
}
