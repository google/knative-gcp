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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
)

const (
	AuthenticationCheckUnknownReason = "AuthenticationCheckPending"
	controlPlaneNamespace            = "cloud-run-events"
)

type AuthTypeArgs struct {
	Namespace           string
	ServiceAccountName  string
	Secret              *corev1.SecretKeySelector
}

// needs to input a service account name lister and a secret lister
func GetAuthType(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister,
	secretLister corev1listers.SecretLister, args AuthTypeArgs) (string, error) {
	if args.ServiceAccountName != "" {
		GetAuthTypeForWorkloadIdentity(ctx, serviceAccountLister, args)
	}

	if args.Secret != "" {
		GetAuthTypeForSecret(secretLister, args)
	}
}

func GetAuthTypeForWorkloadIdentity(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister, args AuthTypeArgs) (string, error) {
	kServiceAccount, err := serviceAccountLister.ServiceAccounts(args.Namespace).Get(args.ServiceAccountName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return kServiceAccount, nil
		}
	} else if {


	} else {

	}
}

func GetAuthTypeForSecret(secretLister corev1listers.SecretLister, args AuthTypeArgs) (string, error) {
	// Controller doesn't have the permission to check the existence of a secret in namespaces
	// other than the control plane's namespace.
	if args.Namespace != controlPlaneNamespace {
		return "secret", nil
	}

	// If current namespace is control plane's namespace, check the existence of the secret.

}