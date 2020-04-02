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
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

const (
	DefaultSecretName = "google-cloud-key"
	defaultSecretKey  = "key.json"
)

var (
	validation_regexp = regexp.MustCompile(`^[a-z][a-z0-9-]{5,29}@[a-z][a-z0-9-]{5,29}.iam.gserviceaccount.com$`)
)

// DefaultGoogleCloudSecretSelector is the default secret selector used to load
// the creds for the objects that will auth with Google Cloud.
func DefaultGoogleCloudSecretSelector() *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: DefaultSecretName,
		},
		Key: defaultSecretKey,
	}
}

// ValidateCredential checks secret and GCP service account.
func ValidateCredential(secret *corev1.SecretKeySelector, gServiceAccountName string) *apis.FieldError {
	if secret != nil && !equality.Semantic.DeepEqual(secret, &corev1.SecretKeySelector{}) && gServiceAccountName != "" {
		return &apis.FieldError{
			Message: "Can't have spec.googleServiceAccount and spec.secret at the same time",
			Paths:   []string{""},
		}
	} else if secret != nil && !equality.Semantic.DeepEqual(secret, &corev1.SecretKeySelector{}) {
		return validateSecret(secret)
	} else if gServiceAccountName != "" {
		return validateGCPServiceAccount(gServiceAccountName)
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

func validateGCPServiceAccount(gServiceAccountName string) *apis.FieldError {
	// The format of gServiceAccountName is service-account-name@project-id.iam.gserviceaccount.com

	// Service account name must be between 6 and 30 characters (inclusive),
	// must begin with a lowercase letter, and consist of lowercase alphanumeric characters that can be separated by hyphens.

	// Project IDs must start with a lowercase letter and can have lowercase ASCII letters, digits or hyphens,
	// must be between 6 and 30 characters.
	match := validation_regexp.FindStringSubmatch(gServiceAccountName)
	if len(match) == 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf(`invalid value: %s, googleServiceAccount should have format: ^[a-z][a-z0-9-]{5,29}@[a-z][a-z0-9-]{5,29}.iam.gserviceaccount.com$`,
				gServiceAccountName),
			Paths: []string{"googleServiceAccount"},
		}
	}
	return nil
}
