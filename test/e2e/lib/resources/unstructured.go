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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	pkgTest "knative.dev/pkg/test"
)

func IstioServiceEntry(name, namespace string) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "networking.istio.io/v1alpha3",
		"kind":       "ServiceEntry",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"hosts": []string{
				"*.googleapis.com",
				"accounts.google.com",
			},
			"ports": []map[string]interface{}{{
				"number": "443",
				"name":     "https",
				"protocol": "HTTPS",
			}},
			"location": "MESH_EXTERNAL",
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

func ReceiverKService(name, namespace string) *unstructured.Unstructured {
	imageName := "receiver"
	obj := map[string]interface{}{
		"apiVersion": "serving.knative.dev/v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{
						"image": pkgTest.ImagePath(imageName),
					}},
				},
			},
		},
	}
	return &unstructured.Unstructured{Object: obj}
}
