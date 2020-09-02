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

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1beta1.CloudBuildSource) to a higher version of CloudBuildSource.
// Currently, we only support v1 as a higher version.
func (source *CloudBuildSource) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1.CloudBuildSource:
		sink.ObjectMeta = source.ObjectMeta
		// Both v1beta1 and v1 CloudBuildSources implement duck v1 PubSubable.
		sink.Spec.PubSubSpec = source.Spec.PubSubSpec
		sink.Status.PubSubStatus = source.Status.PubSubStatus
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1.CloudBuildSource{}, sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from a higher version of CloudBuildSource to a v1beta1.CloudBuildSource.
// Currently, we only support v1 as a higher version.
func (sink *CloudBuildSource) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1.CloudBuildSource:
		sink.ObjectMeta = source.ObjectMeta
		// Both v1beta1 and v1 CloudBuildSources implement duck v1 PubSubable.
		sink.Spec.PubSubSpec = source.Spec.PubSubSpec
		sink.Status.PubSubStatus = source.Status.PubSubStatus
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1.CloudBuildSource{}, sink)
	}
}
