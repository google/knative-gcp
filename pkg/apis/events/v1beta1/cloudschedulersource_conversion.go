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
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts from a v1beta1.CloudSchedulerSource to a higher version of CloudSchedulerSource.
// Currently, we only support v1 as a higher version.
func (source *CloudSchedulerSource) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1.CloudSchedulerSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.ToV1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.Location = source.Spec.Location
		sink.Spec.Schedule = source.Spec.Schedule
		sink.Spec.Data = source.Spec.Data
		sink.Status.PubSubStatus = convert.ToV1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.JobName = source.Status.JobName
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1.CloudSchedulerSource{}, sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts from a higher version of CloudSchedulerSource to a v1beta1.CloudSchedulerSource.
// Currently, we only support v1 as a higher version.
func (sink *CloudSchedulerSource) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1.CloudSchedulerSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.PubSubSpec = convert.FromV1PubSubSpec(source.Spec.PubSubSpec)
		sink.Spec.Location = source.Spec.Location
		sink.Spec.Schedule = source.Spec.Schedule
		sink.Spec.Data = source.Spec.Data
		sink.Status.PubSubStatus = convert.FromV1PubSubStatus(source.Status.PubSubStatus)
		sink.Status.JobName = source.Status.JobName
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1.CloudSchedulerSource{}, sink)
	}
}
