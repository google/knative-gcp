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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	workloadIdentityKey = "iam.gke.io/gcp-service-account"
)

// GenerateServiceAccountName generates a k8s ServiceAccount name according to GCP ServiceAccount
func GenerateServiceAccountName(gServiceAccount string) string {
	return strings.Split(gServiceAccount, "@")[0]
}

// MakeServiceAccount creates a K8s ServiceAccount object for the Namespace.
func MakeServiceAccount(namespace string, gServiceAccount string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      GenerateServiceAccountName(gServiceAccount),
			Annotations: map[string]string{
				workloadIdentityKey: gServiceAccount,
			},
		},
	}
}

// OwnerReferenceExists checks if a K8s ServiceAccount contains specific ownerReference
func OwnerReferenceExists(kServiceAccount *corev1.ServiceAccount, expect metav1.OwnerReference) bool {
	references := kServiceAccount.OwnerReferences
	for _, reference := range references {
		if reference.Name == expect.Name {
			return true
		}
	}
	return false
}
