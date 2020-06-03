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
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"
)

// MakeIngressDeployment creates the ingress Deployment object.
func MakeIngressDeployment(args IngressArgs) *appsv1.Deployment {
	container := containerTemplate(args.Args)
	// Decorate the container template with ingress port.
	container.Env = append(container.Env, corev1.EnvVar{Name: "PORT", Value: strconv.Itoa(args.Port)})
	container.Ports = append(container.Ports, corev1.ContainerPort{Name: "http", ContainerPort: int32(args.Port)})
	container.LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(args.Port),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 5,
		PeriodSeconds:       2,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
	container.Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("500Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("500Mi"),
			corev1.ResourceCPU:    resource.MustParse("1000m"),
		},
	}
	return deploymentTemplate(args.Args, []corev1.Container{container})
}

// MakeFanoutDeployment creates the fanout Deployment object.
func MakeFanoutDeployment(args FanoutArgs) *appsv1.Deployment {
	container := containerTemplate(args.Args)
	container.Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1000Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1000Mi"),
			corev1.ResourceCPU:    resource.MustParse("1500m"),
		},
	}
	container.LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(8080),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 15,
		PeriodSeconds:       15,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
	return deploymentTemplate(args.Args, []corev1.Container{container})
}

// MakeRetryDeployment creates the retry Deployment object.
func MakeRetryDeployment(args RetryArgs) *appsv1.Deployment {
	container := containerTemplate(args.Args)
	container.Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1500Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1500Mi"),
			corev1.ResourceCPU:    resource.MustParse("1000m"),
		},
	}
	container.LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(8080),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 15,
		PeriodSeconds:       15,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
	return deploymentTemplate(args.Args, []corev1.Container{container})
}

// deploymentTemplate creates a template for data plane deployments.
func deploymentTemplate(args Args, containers []corev1.Container) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.BrokerCell.Namespace,
			Name:            Name(args.BrokerCell.Name, args.ComponentName),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.BrokerCell)},
			Labels:          Labels(args.BrokerCell.Name, args.ComponentName),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: Labels(args.BrokerCell.Name, args.ComponentName)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: Labels(args.BrokerCell.Name, args.ComponentName)},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.ServiceAccountName,
					Volumes: []corev1.Volume{
						{
							Name:         "broker-config",
							VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "broker-targets"}}},
						},
						{
							Name:         "google-broker-key",
							VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "google-broker-key", Optional: &optionalSecretVolume}},
						},
					},
					Containers: containers,
				},
			},
		},
	}
}

// containerTemplate returns a common template for broker data plane containers.
func containerTemplate(args Args) corev1.Container {
	return corev1.Container{
		Image: args.Image,
		Name:  args.ComponentName,
		Env: []corev1.EnvVar{
			{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/var/secrets/google/key.json",
			},
			{
				Name:  system.NamespaceEnvKey,
				Value: system.Namespace(),
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "CONFIG_LOGGING_NAME",
				Value: "config-logging",
			},
			{
				Name:  "CONFIG_OBSERVABILITY_NAME",
				Value: "config-observability",
			},
			{
				// Used for StackDriver only.
				Name:  "METRICS_DOMAIN",
				Value: "knative.dev/internal/eventing",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: int32(args.MetricsPort),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "broker-config",
				MountPath: "/var/run/cloud-run-events/broker",
			},
			{
				Name:      "google-broker-key",
				MountPath: "/var/secrets/google",
			},
		},
	}
}
