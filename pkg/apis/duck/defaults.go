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

// Package duck contains Cloud Run Events API versions for duck components
package duck

import (
	"context"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetAutoscalingAnnotationsDefaults(ctx context.Context, obj *metav1.ObjectMeta) {
	// If autoscaling was configured, then set defaults.
	if _, ok := obj.Annotations[AutoscalingClassAnnotation]; ok {
		setDefaultAnnotationIfNotPresent(obj, AutoscalingMinScaleAnnotation, defaultMinScale)
		setDefaultAnnotationIfNotPresent(obj, AutoscalingMaxScaleAnnotation, defaultMaxScale)
		setDefaultAnnotationIfNotPresent(obj, KedaAutoscalingPollingIntervalAnnotation, defaultKedaPollingInterval)
		setDefaultAnnotationIfNotPresent(obj, KedaAutoscalingCooldownPeriodAnnotation, defaultKedaCooldownPeriod)
		setDefaultAnnotationIfNotPresent(obj, KedaAutoscalingSubscriptionSizeAnnotation, defaultKedaSubscriptionSize)
		// If it wasn't configured, then delete any autoscaling related configuration.
	} else {
		deleteAnnotationIfPresent(obj, AutoscalingMinScaleAnnotation)
		deleteAnnotationIfPresent(obj, AutoscalingMaxScaleAnnotation)
		deleteAnnotationIfPresent(obj, KedaAutoscalingPollingIntervalAnnotation)
		deleteAnnotationIfPresent(obj, KedaAutoscalingCooldownPeriodAnnotation)
		deleteAnnotationIfPresent(obj, KedaAutoscalingSubscriptionSizeAnnotation)
	}
}

func setDefaultAnnotationIfNotPresent(obj *metav1.ObjectMeta, annotation string, defaultValue string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	if _, ok := obj.Annotations[annotation]; !ok {
		obj.Annotations[annotation] = defaultValue
	}
}

func deleteAnnotationIfPresent(obj *metav1.ObjectMeta, annotation string) {
	if _, ok := obj.Annotations[annotation]; ok {
		delete(obj.Annotations, annotation)
	}
}

// SetClusterNameAnnotation sets the cluster-name annotation when running on GKE or GCE.
func SetClusterNameAnnotation(obj *metav1.ObjectMeta, client metadataClient.Client) {
	if _, ok := obj.Annotations[ClusterNameAnnotation]; !ok && client.OnGCE() {
		clusterName, err := utils.ClusterName(obj.Annotations[ClusterNameAnnotation], client)
		// If metadata access is disabled for some reason, leave the annotation to be empty.
		if err == nil {
			setDefaultAnnotationIfNotPresent(obj, ClusterNameAnnotation, clusterName)
		}
	}
}
