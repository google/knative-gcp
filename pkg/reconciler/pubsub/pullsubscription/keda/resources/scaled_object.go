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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strconv"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"k8s.io/api/apps/v1"
)

var (
	ScaledObjectGVK = schema.GroupVersionKind{
		Group:   "keda.k8s.io",
		Version: "v1alpha1",
		Kind:    "ScaledObject",
	}

	KedaSchemeGroupVersion = schema.GroupVersion{Group: "keda.k8s.io", Version: "v1alpha1"}
)

func MakeScaledObject(ctx context.Context, ra *v1.Deployment, ps *v1alpha1.PullSubscription) *unstructured.Unstructured {
	// These values should have already been validated in the webhook, and be valid ints. Not checking for errors.
	minReplicaCount, _ := strconv.Atoi(ps.Annotations[duckv1alpha1.AutoscalingMinScaleAnnotation])
	maxReplicateCount, _ := strconv.Atoi(ps.Annotations[duckv1alpha1.AutoscalingMaxScaleAnnotation])
	cooldownPeriod, _ := strconv.Atoi(ps.Annotations[duckv1alpha1.KedaAutoscalingCooldownPeriodAnnotation])
	pollingInterval, _ := strconv.Atoi(ps.Annotations[duckv1alpha1.KedaAutoscalingPollingIntervalAnnotation])

	so := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.k8s.io/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"namespace": ra.Namespace,
				"name":      GenerateScaledObjectName(ra),
				"labels": map[string]interface{}{
					"deploymentName":                  ra.Name,
					"events.cloud.google.com/ps-name": ps.Name,
				},
				// Make the Deployment the owner. The PS in turn is the owner of the Deployment.
				// The deletions will cascade. The good thing of making the Deployment the owner
				// is that we will recreate a new ScaledObject if someone deletes the Deployment.
				"ownerReferences": []map[string]interface{}{{
					"apiVersion":         "apps/v1",
					"kind":               "Deployment",
					"blockOwnerDeletion": true,
					"controller":         true,
					"name":               ra.Name,
					"uid":                ra.UID,
				}},
			},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"deploymentName": ra.Name,
				},
				"minReplicaCount": minReplicaCount,
				"maxReplicaCount": maxReplicateCount,
				"cooldownPeriod":  cooldownPeriod,
				"pollingInterval": pollingInterval,
				"triggers": []map[string]interface{}{{
					"type": "gcp-pubsub",
					"metadata": map[string]interface{}{
						"subscriptionSize": ps.Annotations[duckv1alpha1.KedaAutoscalingSubscriptionSizeAnnotation],
						"subscriptionName": ps.Status.SubscriptionID,
						"credentials":      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
					},
				}},
			},
		},
	}
	return so
}
