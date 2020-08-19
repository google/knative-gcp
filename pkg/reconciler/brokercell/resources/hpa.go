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
	appsv1 "k8s.io/api/apps/v1"
	hpav2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

// MakeHorizontalPodAutoscaler makes an HPA for the given arguments.
func MakeHorizontalPodAutoscaler(deployment *appsv1.Deployment, args AutoscalingArgs) *hpav2beta2.HorizontalPodAutoscaler {
	autoscalingMetrics := []hpav2beta2.MetricSpec{}
	if args.AvgCPUUtilization != nil {
		cpuMetric := hpav2beta2.MetricSpec{
			Type: hpav2beta2.ResourceMetricSourceType,
			Resource: &hpav2beta2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: hpav2beta2.MetricTarget{
					Type:               hpav2beta2.UtilizationMetricType,
					AverageUtilization: args.AvgCPUUtilization,
				},
			},
		}
		autoscalingMetrics = append(autoscalingMetrics, cpuMetric)
	}
	if args.AvgMemoryUsage != nil {
		if memQuantity, err := resource.ParseQuantity(*args.AvgMemoryUsage); err == nil {
			memoryMetric := hpav2beta2.MetricSpec{
				Type: hpav2beta2.ResourceMetricSourceType,
				Resource: &hpav2beta2.ResourceMetricSource{
					Name: corev1.ResourceMemory,
					Target: hpav2beta2.MetricTarget{
						Type:         hpav2beta2.AverageValueMetricType,
						AverageValue: &memQuantity,
					},
				},
			}
			autoscalingMetrics = append(autoscalingMetrics, memoryMetric)
		}
	}

	return &hpav2beta2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deployment.Name + "-hpa",
			Namespace:       deployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.BrokerCell)},
			Labels:          Labels(args.BrokerCell.Name, args.ComponentName),
		},
		Spec: hpav2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: hpav2beta2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       deployment.Name,
			},
			MaxReplicas: args.MaxReplicas,
			MinReplicas: &args.MinReplicas,
			Metrics:     autoscalingMetrics,
		},
	}
}
