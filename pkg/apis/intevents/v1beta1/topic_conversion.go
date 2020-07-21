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
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/convert"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts a v1beta1.Topic to a higher version of topic.
// Currently, we only support v1 as a higher version.
func (source *Topic) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1.Topic:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.IdentitySpec = convert.ToV1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Topic = source.Spec.Topic
		if pp, err := convertToV1PropagationPolicy(source.Spec.PropagationPolicy); err != nil {
			return err
		} else {
			sink.Spec.PropagationPolicy = pp
		}
		sink.Spec.EnablePublisher = source.Spec.EnablePublisher
		sink.Status.IdentityStatus = convert.ToV1IdentityStatus(source.Status.IdentityStatus)
		if as, err := convert.ToV1AddressStatus(ctx, source.Status.AddressStatus); err != nil {
			return err
		} else {
			sink.Status.AddressStatus = as
		}
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1.Topic{}, sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts from a higher version of Topic to v1beta1.Topic.
// Currently, we only support v1 as a higher version.
func (sink *Topic) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1.Topic:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.IdentitySpec = convert.FromV1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Topic = source.Spec.Topic
		if pp, err := convertFromV1beta1PropagationPolicy(source.Spec.PropagationPolicy); err != nil {
			return err
		} else {
			sink.Spec.PropagationPolicy = pp
		}
		sink.Spec.EnablePublisher = source.Spec.EnablePublisher
		sink.Status.IdentityStatus = convert.FromV1IdentityStatus(source.Status.IdentityStatus)
		if as, err := convert.FromV1AddressStatus(ctx, source.Status.AddressStatus); err != nil {
			return err
		} else {
			sink.Status.AddressStatus = as
		}
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1.Topic{}, sink)
	}
}

func convertToV1PropagationPolicy(pp PropagationPolicyType) (v1.PropagationPolicyType, error) {
	switch pp {
	case TopicPolicyCreateDelete:
		return v1.TopicPolicyCreateDelete, nil
	case TopicPolicyCreateNoDelete:
		return v1.TopicPolicyCreateNoDelete, nil
	case TopicPolicyNoCreateNoDelete:
		return v1.TopicPolicyNoCreateNoDelete, nil
	case "":
		return "", nil
	default:
		return "unknown", fmt.Errorf("unknown PropagationPolicyType %v", pp)
	}
}

func convertFromV1beta1PropagationPolicy(pp v1.PropagationPolicyType) (PropagationPolicyType, error) {
	switch pp {
	case v1.TopicPolicyCreateDelete:
		return TopicPolicyCreateDelete, nil
	case v1.TopicPolicyCreateNoDelete:
		return TopicPolicyCreateNoDelete, nil
	case v1.TopicPolicyNoCreateNoDelete:
		return TopicPolicyNoCreateNoDelete, nil
	case "":
		return "", nil
	default:
		return "unknown", fmt.Errorf("unknown PropagationPolicyType %v", pp)
	}
}
