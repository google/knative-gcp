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

	"github.com/google/knative-gcp/pkg/apis/convert"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1beta1.PullSubscription) into v1alpha1.PullSubscription.
func (source *Channel) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.Channel:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.IdentitySpec = convert.ToV1beta1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Subscribable = convert.ToV1beta1SubscribableSpec(source.Spec.Subscribable)
		sink.Status.IdentityStatus = convert.ToV1beta1IdentityStatus(source.Status.IdentityStatus)
		sink.Status.AddressStatus = source.Status.AddressStatus
		sink.Status.SubscribableStatus = convert.ToV1beta1SubscribableStatus(source.Status.SubscribableTypeStatus)
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha1.PullSubscription into v1beta1.PullSubscription.
func (sink *Channel) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.Channel:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.IdentitySpec = convert.FromV1beta1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Subscribable = convert.FromV1beta1SubscribableSpec(source.Spec.Subscribable)
		sink.Status.IdentityStatus = convert.FromV1beta1IdentityStatus(source.Status.IdentityStatus)
		sink.Status.AddressStatus = source.Status.AddressStatus
		sink.Status.SubscribableTypeStatus = convert.FromV1beta1SubscribableStatus(source.Status.SubscribableStatus)
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", source)
	}
}
