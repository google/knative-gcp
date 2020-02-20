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

package resources

import (
	"context"
	"encoding/json"
	"knative.dev/pkg/kmeta"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/ptr"
	"strconv"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	kedav1alpha1 "github.com/kedacore/keda/pkg/apis/keda/v1alpha1"
	"k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ScaledObjectGVK = schema.GroupVersionKind{
		Group:   kedav1alpha1.SchemeGroupVersion.Group,
		Version: kedav1alpha1.SchemeGroupVersion.Version,
		Kind:    "ScaledObject",
	}

	KedaSchemeGroupVersion = kedav1alpha1.SchemeGroupVersion
)

func MakeScaledObject(ctx context.Context, ra *v1.Deployment, ps *v1alpha1.PullSubscription) (*unstructured.Unstructured, error) {

	// These values should have already been validated in the webhook, and be valid ints. Not checking for errors.
	minReplicaCount, _ := strconv.ParseInt(ps.Annotations[duckv1alpha1.AutoscalingMinScaleAnnotation], 10, 64)
	maxReplicateCount, _ := strconv.ParseInt(ps.Annotations[duckv1alpha1.AutoscalingMaxScaleAnnotation], 10, 64)
	cooldownPeriod, _ := strconv.ParseInt(ps.Annotations[duckv1alpha1.KedaAutoscalingCooldownPeriodAnnotation], 10, 64)
	pollingInterval, _ := strconv.ParseInt(ps.Annotations[duckv1alpha1.KedaAutoscalingPollingIntervalAnnotation], 10, 64)

	apiVersion, kind := ScaledObjectGVK.ToAPIVersionAndKind()

	so := kedav1alpha1.ScaledObject{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ra.Namespace,
			Name:      GenerateScaledObjectName(ps),
			Labels: map[string]string{
				"deploymentName":                  ra.Name,
				"events.cloud.google.com/ps-name": ps.Name,
			},
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(ps)},
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ObjectReference{
				DeploymentName: ra.Name,
			},
			MinReplicaCount: ptr.Int32(int32(minReplicaCount)),
			MaxReplicaCount: ptr.Int32(int32(maxReplicateCount)),
			CooldownPeriod:  ptr.Int32(int32(cooldownPeriod)),
			PollingInterval: ptr.Int32(int32(pollingInterval)),
			Triggers: []kedav1alpha1.ScaleTriggers{{
				Type: "gcp-pubsub",
				Metadata: map[string]string{
					"subscriptionSize": ps.Annotations[duckv1alpha1.KedaAutoscalingSubscriptionSizeAnnotation],
					"subscriptionName": ps.Status.SubscriptionID,
					"credentials":      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
				},
			}},
		},
	}

	raw, err := json.Marshal(so)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = json.Unmarshal(raw, u)
	if err != nil {
		return nil, err
	}
	return u, nil
}
