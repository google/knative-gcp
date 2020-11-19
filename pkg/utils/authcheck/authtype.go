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

// Package authcheck provides utilities to check authentication configuration for data plane resources.
// File authtype contains functions to differentiate authentication mode.
package authcheck

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
)

type AuthType string

const (
	// Secret option is referring to authentication configuration for secret.
	// https://cloud.google.com/kubernetes-engine/docs/tutorials/authenticating-to-cloud-platform#importing_credentials_as_a_secret
	Secret AuthType = "secret"
	// WorkloadIdentityGSA option is referring to authentication configuration for Workload Identity using GSA
	// https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity
	WorkloadIdentityGSA AuthType = "workload-identity-gsa"
	WorkloadIdentity    AuthType = "workload-identity"
)

type AuthTypeArgs struct {
	Namespace          string
	ServiceAccountName string
	Secret             *corev1.SecretKeySelector
}

const (
	AuthenticationCheckUnknownReason = "AuthenticationCheckPending"
	ControlPlaneNamespace            = "events-system"
	BrokerServiceAccountName         = "broker"
)

var (
	// Regex for a valid google service account email.
	// The format of google service account email is service-account-name@project-id.iam.gserviceaccount.com
	// Service account name must be between 6 and 30 characters (inclusive),
	// must begin with a lowercase letter, and consist of lowercase alphanumeric characters that can be separated by hyphens.
	// Project IDs must start with a lowercase letter and can have lowercase ASCII letters, digits or hyphens,
	// must be between 6 and 30 characters. Some older project may have dot as well, like project-example.example.com
	emailRegexp = regexp.MustCompile(`^[a-z][a-z0-9-]{5,29}@[a-z][a-z0-9-]{5,29}.*\.iam\.gserviceaccount\.com$`)

	BrokerSecret = &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "google-broker-key"},
		Key:                  "key.json",
	}
)

// GetAuthTypeForBrokerCell will get authType for BrokerCell.
func GetAuthTypeForBrokerCell(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister,
	secretLister corev1listers.SecretLister, args AuthTypeArgs) (AuthType, error) {
	// For AuthTypeArgs from BrokerCell, ServiceAccountName and Secret will be both presented.
	// We need to revisit this function after https://github.com/google/knative-gcp/issues/1888 lands,
	// which will add IdentitySpec to BrokerCell.
	// For AuthTypeArgs from BrokerCell.
	authTypeForWorkloadIdentity, workloadIdentityErr := getAuthTypeForWorkloadIdentity(ctx, serviceAccountLister, args)
	if workloadIdentityErr == nil {
		return authTypeForWorkloadIdentity, nil
	}
	authTypeForSecret, secretErr := getAuthTypeForSecret(ctx, secretLister, args)
	if secretErr == nil {
		return authTypeForSecret, nil
	}

	workloadIdentityError := fmt.Errorf("when checking Kubernetes Service Account %s, got error: %w", args.ServiceAccountName, workloadIdentityErr)
	secretError := fmt.Errorf("when checking Kubernetes Secret %s, got error: %w", args.Secret.Name, secretErr)
	return "", fmt.Errorf("authentication is not configured, %s, %s", workloadIdentityError.Error(), secretError.Error())
}

// GetAuthTypeForSources will get authType for Sources.
func GetAuthTypeForSources(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister, args AuthTypeArgs) (AuthType, error) {
	// For AuthTypeArgs from Sources, either ServiceAccountName or Secret will be empty,
	// because of the IdentitySpec validation from Webhook.

	// For AuthTypeArgs from Sources which has serviceAccountName.
	if args.ServiceAccountName != "" {
		authType, err := getAuthTypeForWorkloadIdentity(ctx, serviceAccountLister, args)
		if err != nil {
			return "", fmt.Errorf("using Workload Identity for authentication configuration: %w", err)
		}
		return authType, nil
	}

	// For AuthTypeArgs from Sources which has secret.
	if args.Secret != nil {
		// Sources' secrets are not further checked.
		// In most cases, sources don't live in the control plane's namespace,
		// and the controller doesn't have the permission to check their secrets.
		return Secret, nil
	}

	return "", errors.New("invalid AuthTypeArgs, neither ServiceAccountName nor Secret are provided")
}

func getAuthTypeForWorkloadIdentity(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister,
	args AuthTypeArgs) (AuthType, error) {
	kServiceAccount, err := serviceAccountLister.ServiceAccounts(args.Namespace).Get(args.ServiceAccountName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return "", fmt.Errorf("can't find Kubernetes Service Account %s",
				args.ServiceAccountName)
		}
		return "", fmt.Errorf("error getting Kubernetes Service Account: %w", err)
	} else if email := kServiceAccount.Annotations[resources.WorkloadIdentityKey]; email != "" {
		// Check if email is a valid google service account email.
		if match := emailRegexp.FindString(email); match == "" {
			return "", fmt.Errorf("%s is not a valid Google Service Account as the value of Kubernetes Service Account %s for annotation %s",
				email, args.ServiceAccountName, resources.WorkloadIdentityKey)
		}
		return WorkloadIdentityGSA, nil
	}
	// Once workload-identity new gen lands, we should also include the annotation check for it.
	return "", fmt.Errorf("the Kubernetes Service Account %s does not have the required annotation", args.ServiceAccountName)
}

func getAuthTypeForSecret(ctx context.Context, secretLister corev1listers.SecretLister, args AuthTypeArgs) (AuthType, error) {
	// Controller doesn't have the permission to check the existence of a secret in namespaces
	// other than the control plane's namespace.
	if args.Namespace != ControlPlaneNamespace {
		return Secret, nil
	}
	// If current namespace is control plane's namespace, check the existence of the secret and its key.
	secret, err := secretLister.Secrets(args.Namespace).Get(args.Secret.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return "", fmt.Errorf("can't find Kubernetes Secret %v",
				args.Secret.Name)
		}
		return "", fmt.Errorf("error getting Kubernetes Secret: %w", err)
	} else if secret.Data[args.Secret.Key] == nil {
		return "", fmt.Errorf("the Kubernetes Secret %s does not have required key %s",
			args.Secret.Name, args.Secret.Key)
	}
	return Secret, nil
}
