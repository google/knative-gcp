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

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1beta1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1beta1.PullSubscription) into v1alpha1.PullSubscription.
func (source *Topic) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta1.Topic:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.IdentitySpec = convertToV1beta1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Topic = source.Spec.Topic
		if pp, err := convertToV1beta1PropagationPolicy(source.Spec.PropagationPolicy); err != nil {
			return err
		} else {
			sink.Spec.PropagationPolicy = pp
		}
		sink.Status.IdentityStatus = convertToV1beta1IdentityStatus(source.Status.IdentityStatus)
		if as, err := convertToV1beta1AddressStatus(ctx, source.Status.AddressStatus); err != nil {
			return err
		} else {
			sink.Status.AddressStatus = as
		}
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha1.PullSubscription into v1beta1.PullSubscription.
func (sink *Topic) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta1.Topic:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.IdentitySpec = convertFromV1beta1IdentitySpec(source.Spec.IdentitySpec)
		sink.Spec.Secret = source.Spec.Secret
		sink.Spec.Project = source.Spec.Project
		sink.Spec.Topic = source.Spec.Topic
		if pp, err := convertFromV1beta1PropagationPolicy(source.Spec.PropagationPolicy); err != nil {
			return err
		} else {
			sink.Spec.PropagationPolicy = pp
		}
		sink.Status.IdentityStatus = convertFromV1beta1IdentityStatus(source.Status.IdentityStatus)
		if as, err := convertFromV1beta1AddressStatus(ctx, source.Status.AddressStatus); err != nil {
			return err
		} else {
			sink.Status.AddressStatus = as
		}
		sink.Status.ProjectID = source.Status.ProjectID
		sink.Status.TopicID = source.Status.TopicID
		return nil
	default:
		return fmt.Errorf("unknown conversion, got: %T", source)
	}
}

func convertToV1beta1PropagationPolicy(pp PropagationPolicyType) (v1beta1.PropagationPolicyType, error) {
	switch pp {
	case TopicPolicyCreateDelete:
		return v1beta1.TopicPolicyCreateDelete, nil
	case TopicPolicyCreateNoDelete:
		return v1beta1.TopicPolicyCreateNoDelete, nil
	case TopicPolicyNoCreateNoDelete:
		return v1beta1.TopicPolicyNoCreateNoDelete, nil
	case "":
		return "", nil
	default:
		return "unknown", fmt.Errorf("unknown PropagationPolicyType %v", pp)
	}
}

func convertFromV1beta1PropagationPolicy(pp v1beta1.PropagationPolicyType) (PropagationPolicyType, error) {
	switch pp {
	case v1beta1.TopicPolicyCreateDelete:
		return TopicPolicyCreateDelete, nil
	case v1beta1.TopicPolicyCreateNoDelete:
		return TopicPolicyCreateNoDelete, nil
	case v1beta1.TopicPolicyNoCreateNoDelete:
		return TopicPolicyNoCreateNoDelete, nil
	case "":
		return "", nil
	default:
		return "unknown", fmt.Errorf("unknown PropagationPolicyType %v", pp)
	}
}

func convertToV1beta1AddressStatus(ctx context.Context, from v1alpha1.AddressStatus) (duckv1beta1.AddressStatus, error) {
	to := duckv1beta1.AddressStatus{}
	if from.Address != nil {
		to.Address = &duckv1beta1.Addressable{}
		err := from.Address.ConvertTo(ctx, to.Address)
		if err != nil {
			return duckv1beta1.AddressStatus{}, err
		}
	}
	return to, nil
}

func convertFromV1beta1AddressStatus(ctx context.Context, from duckv1beta1.AddressStatus) (v1alpha1.AddressStatus, error) {
	to := v1alpha1.AddressStatus{}
	if from.Address != nil {
		to.Address = &v1alpha1.Addressable{}
		err := to.Address.ConvertFrom(ctx, from.Address)
		if err != nil {
			return v1alpha1.AddressStatus{}, err
		}
	}
	return to, nil
}
