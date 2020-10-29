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

// Package testingdata creates data plane resources used by tests by reading YAML files.
// This reduces the cubersome of creating those resources in go code. The YAML
// files are also easier to read.
package testingdata

import (
	"io/ioutil"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	hpav2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func IngressDeployment(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/ingress_deployment.yaml")
}

// TODO(1804): remove this function when ingress filtering is enabled by default.
func IngressDeploymentWithFilteringAnnotation(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/ingress_deployment_with_filtering_annotation.yaml")
}

func FanoutDeployment(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/fanout_deployment.yaml")
}

func RetryDeployment(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/retry_deployment.yaml")
}

func IngressService(t *testing.T) *corev1.Service {
	return getService(t, "testingdata/ingress_service.yaml")
}

func IngressDeploymentWithStatus(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/ingress_deployment_with_status.yaml")
}

func FanoutDeploymentWithStatus(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/fanout_deployment_with_status.yaml")
}

func RetryDeploymentWithStatus(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/retry_deployment_with_status.yaml")
}

func IngressDeploymentWithRestartAnnotation(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/ingress_deployment_with_restart_annotation.yaml")
}

func FanoutDeploymentWithRestartAnnotation(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/fanout_deployment_with_restart_annotation.yaml")
}

func RetryDeploymentWithRestartAnnotation(t *testing.T) *appsv1.Deployment {
	return getDeployment(t, "testingdata/retry_deployment_with_restart_annotation.yaml")
}

func IngressServiceWithStatus(t *testing.T) *corev1.Service {
	return getService(t, "testingdata/ingress_service_with_status.yaml")
}

func IngressHPA(t *testing.T) *hpav2beta2.HorizontalPodAutoscaler {
	return getHPA(t, "testingdata/ingress_hpa.yaml")
}

func FanoutHPA(t *testing.T) *hpav2beta2.HorizontalPodAutoscaler {
	return getHPA(t, "testingdata/fanout_hpa.yaml")
}

func RetryHPA(t *testing.T) *hpav2beta2.HorizontalPodAutoscaler {
	return getHPA(t, "testingdata/retry_hpa.yaml")
}

func getHPA(t *testing.T, path string) *hpav2beta2.HorizontalPodAutoscaler {
	hpa := &hpav2beta2.HorizontalPodAutoscaler{}
	if err := getSpecFromFile(path, hpa); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	return hpa
}

func getDeployment(t *testing.T, path string) *appsv1.Deployment {
	d := &appsv1.Deployment{}
	if err := getSpecFromFile(path, d); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	return d
}

func getService(t *testing.T, path string) *corev1.Service {
	s := &corev1.Service{}
	if err := getSpecFromFile(path, s); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	return s
}

func getSpecFromFile(path string, spec interface{}) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(bytes, spec)
}
