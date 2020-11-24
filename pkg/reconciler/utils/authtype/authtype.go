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

package authtype

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
)

const (
	AuthenticationCheckUnknownReason = "AuthenticationCheckPending"
	ControlPlaneNamespace            = "cloud-run-events"
	BrokerServiceAccountName         = "broker"
)

var BrokerSecret = &corev1.SecretKeySelector{
	LocalObjectReference: corev1.LocalObjectReference{
		Name: "google-broker-key",
	},
	Key: "key.json",
}

type AuthTypeArgs struct {
	Namespace          string
	ServiceAccountName string
	Secret             *corev1.SecretKeySelector
}

func GetAuthType(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister,
	secretLister corev1listers.SecretLister, args AuthTypeArgs) (string, error) {
	// For AuthTypeArgs from Sources, either ServiceAccountName or Secret will be empty,
	// because of the IdentitySpec validation from Webhook.
	// For AuthTypeArgs from BrokerCell, ServiceAccountName and Secret will be both presented.
	// We need to revisit this function after https://github.com/google/knative-gcp/issues/1888 lands,
	// which will add IdentitySpec to BrokerCell.
	// For AuthTypeArgs from BrokerCell.
	if args.ServiceAccountName != "" && args.Secret != nil {
		if authType, err := GetAuthTypeForSecret(ctx, secretLister, args); authType != "" {
			return authType, err
		} else if authType, err := GetAuthTypeForWorkloadIdentity(ctx, serviceAccountLister, args); authType != "" {
			return authType, err
		} else {
			return "", fmt.Errorf("authentication is not configured, Secret doesn't present, ServiceAccountName doesn't have required annotation")
		}
	}

	// For AuthTypeArgs from Sources which has serviceAccountName.
	if args.ServiceAccountName != "" {
		return GetAuthTypeForWorkloadIdentity(ctx, serviceAccountLister, args)
	}

	// For AuthTypeArgs from Sources which has secret.
	if args.Secret != nil {
		return GetAuthTypeForSecret(ctx, secretLister, args)
	}

	return "", fmt.Errorf("invalid AuthTypeArgs, neither ServiceAccountName nor Secret are provided")
}

func GetAuthTypeForWorkloadIdentity(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister, args AuthTypeArgs) (string, error) {
	kServiceAccount, err := serviceAccountLister.ServiceAccounts(args.Namespace).Get(args.ServiceAccountName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return "", fmt.Errorf("using Workload Identity for authentication configuration, " +
				"can't find Kubernetes Service Account " + args.ServiceAccountName)
		}
		return "workload-identity", fmt.Errorf("error getting Kubernests Service Account: %s", err.Error())
	} else if kServiceAccount.Annotations[resources.WorkloadIdentityKey] != "" {
		return "workload-identity-gsa", nil
	}
	// Once workload-identity-ubermint lands, we should also include the annotation check for it.
	return "", fmt.Errorf("using Workload Identity for authentication configuration, " +
		"Kubernetes Service Account " + args.ServiceAccountName + " doesn't have the required annotation")
}

func GetAuthTypeForSecret(ctx context.Context, secretLister corev1listers.SecretLister, args AuthTypeArgs) (string, error) {
	// Controller doesn't have the permission to check the existence of a secret in namespaces
	// other than the control plane's namespace.
	if args.Namespace != ControlPlaneNamespace {
		return "secret", nil
	}
	// If current namespace is control plane's namespace, check the existence of the secret and its key.
	secret, err := secretLister.Secrets(args.Namespace).Get(args.Secret.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return "", fmt.Errorf("using Secret for authentication configuration, " +
				"can't find Kubernests Secret " + args.Secret.Name)
		}
		return "secret", fmt.Errorf("error getting Kubernests Secret: %s", err.Error())
	} else if secret.Data[args.Secret.Key] == nil {
		return "secret", fmt.Errorf("using Secret for authentication configuration, " +
			"Kubernests Secret " + args.Secret.Name + " doesn't have required key " + args.Secret.Key)
	}
	return "secret", nil
}
