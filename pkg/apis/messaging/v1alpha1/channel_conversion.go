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
	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1beta1.Channel) into v1alpha1.Channel.
func (source *Channel) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.Channel:
		sink.ObjectMeta = source.ObjectMeta
		if sink.Annotations == nil {
			sink.Annotations = make(map[string]string, 1)
		}
		sink.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1beta1"
		// v1beta1 Channel implements duck v1 identifiable
		sink.Spec.IdentitySpec = convert.FromV1alpha1ToV1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.SubscribableSpec = convert.ToV1beta1SubscribableSpec(source.Spec.Subscribable)
		// v1beta1 Channel implements duck v1 identifiable
		sink.Status.IdentityStatus = convert.FromV1alpha1ToV1IdentityStatus(source.Status.IdentityStatus)
		sink.Status.AddressStatus = source.Status.AddressStatus
		source.Status.SubscribableTypeStatus.ConvertTo(ctx, &sink.Status.SubscribableStatus)
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		// Remove v1alpha1 as deprecated from the Status Condition when converting to a higher version.
		convert.RemoveV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha1.Channel into v1beta1.Channel.
func (sink *Channel) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.Channel:
		sink.ObjectMeta = source.ObjectMeta
		if sink.Annotations == nil {
			sink.Annotations = make(map[string]string, 1)
		}
		sink.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1alpha1"
		// v1beta1 Channel implements duck v1 identifiable
		sink.Spec.IdentitySpec = convert.FromV1ToV1alpha1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Subscribable = convert.FromV1beta1SubscribableSpec(source.Spec.SubscribableSpec)
		// v1beta1 Channel implements duck v1 identifiable
		sink.Status.IdentityStatus = convert.FromV1ToV1alpha1IdentityStatus(source.Status.IdentityStatus)
		sink.Status.AddressStatus = source.Status.AddressStatus
		if err := sink.Status.SubscribableTypeStatus.ConvertFrom(ctx, &source.Status.SubscribableStatus); err != nil {
			return err
		}
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		// Mark v1alpha1 as deprecated as a Status Condition when converting to v1alpha1.
		convert.MarkV1alpha1Deprecated(sink.ConditionSet(), &sink.Status.Status)
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", source)
	}
}
