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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	WorkloadIdentityKey = "iam.gke.io/gcp-service-account"
)

// GenerateServiceAccountName generates a k8s ServiceAccount name according to GCP ServiceAccount
func GenerateServiceAccountName(kServiceAccount, clusterName string) string {
	return fmt.Sprintf("%s-%s", kServiceAccount, clusterName)
}

// MakeServiceAccount creates a K8s ServiceAccount object for the Namespace.
func MakeServiceAccount(namespace string, kServiceAccount, gServiceAccount, clusterName string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      GenerateServiceAccountName(kServiceAccount, clusterName),
			Annotations: map[string]string{
				WorkloadIdentityKey: gServiceAccount,
			},
		},
	}
}
