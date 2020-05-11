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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"knative.dev/pkg/kmeta"
)

// MakeIngressService creates the ingress Service.
func MakeIngressService(args IngressArgs) *corev1.Service {
	bc := args.BrokerCell
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       bc.Namespace,
			Name:            Name(bc.Name, args.ComponentName),
			Labels:          Labels(bc.Name, args.ComponentName),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(bc)},
		},
		Spec: corev1.ServiceSpec{
			Selector: Labels(bc.Name, args.ComponentName),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(args.Port),
				},
				{
					Name: "http-metrics",
					Port: int32(args.MetricsPort),
				},
			},
		},
	}
}
