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

package v1alpha1

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

const (
	DefaultSecretName = "google-cloud-key"
)

var (
	// The name of a k8s ServiceAccount object must be a valid DNS subdomain name.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
	ksaValidationRegex = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`)
)

// ValidateCredential checks secret and service account.
func ValidateCredential(secret *corev1.SecretKeySelector, kServiceAccountName string) *apis.FieldError {
	if secret != nil && kServiceAccountName != "" {
		return &apis.FieldError{
			Message: "Can't have spec.serviceAccountName and spec.secret at the same time",
			Paths:   []string{""},
		}
	} else if secret != nil {
		return validateSecret(secret)
	} else if kServiceAccountName != "" {
		return validateK8sServiceAccount(kServiceAccountName)
	}
	return nil
}

func validateSecret(secret *corev1.SecretKeySelector) *apis.FieldError {
	var errs *apis.FieldError
	if secret.Name == "" {
		errs = errs.Also(apis.ErrMissingField("secret.name"))
	}
	if secret.Key == "" {
		errs = errs.Also(apis.ErrMissingField("secret.key"))
	}
	return errs
}

func validateK8sServiceAccount(kServiceAccountName string) *apis.FieldError {
	match := ksaValidationRegex.FindStringSubmatch(kServiceAccountName)
	if len(match) == 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf(`invalid value: %s, serviceAccountName should have format: ^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`,
				kServiceAccountName),
			Paths: []string{"serviceAccountName"},
		}
	}
	return nil
}
