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

package identity

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/google/knative-gcp/pkg/duck"
	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
)

type Identity struct {
	// For dealing with Topics and Pullsubscriptions
	KubeClient kubernetes.Interface
}

func (i *Identity) ReconcileWorkloadIdentity(ctx context.Context, projectID, namespace string, identifiable duck.Identifiable) (*corev1.ServiceAccount, error) {
	// Create corresponding k8s ServiceAccount if it doesn't exist.
	kServiceAccount, err := i.createServiceAccount(ctx, projectID, namespace, identifiable.GetIdentity())
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s ServiceAccount: %w", err)
	}
	// Add ownerReference to K8s ServiceAccount.
	expectOwnerReference := *kmeta.NewControllerRef(identifiable)
	expectOwnerReference.Controller = ptr.Bool(false)
	if !resources.OwnerReferenceExists(kServiceAccount, expectOwnerReference) {
		kServiceAccount.OwnerReferences = append(kServiceAccount.OwnerReferences, expectOwnerReference)
		if _, err := i.KubeClient.CoreV1().ServiceAccounts(kServiceAccount.Namespace).Update(kServiceAccount); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update OwnerReferences", zap.Error(err))
			return nil, fmt.Errorf("failed to update OwnerReferences: %w", err)
		}
	}

	// Add iam policy binding to GCP ServiceAccount.
	if err := resources.AddIamPolicyBinding(ctx, projectID, identifiable.GetIdentity(), kServiceAccount); err != nil {
		return kServiceAccount, fmt.Errorf("adding iam policy binding failed with: %s", err)
	}
	return kServiceAccount, nil
}

func (i *Identity) createServiceAccount(ctx context.Context, projectID, namespace, gServiceAccount string) (*corev1.ServiceAccount, error) {
	kServiceAccountName := resources.GenerateServiceAccountName(gServiceAccount)
	kServiceAccount, err := i.KubeClient.CoreV1().ServiceAccounts(namespace).Get(kServiceAccountName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			expect := resources.MakeServiceAccount(namespace, gServiceAccount)
			logging.FromContext(ctx).Desugar().Debug("Creating k8s service account", zap.Any("ksa", expect))
			kServiceAccount, err := i.KubeClient.CoreV1().ServiceAccounts(expect.Namespace).Create(expect)
			if err != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to create k8s service account", zap.Error(err))
				return nil, fmt.Errorf("failed to create k8s service account: %w", err)
			}
			return kServiceAccount, nil
		}
		logging.FromContext(ctx).Desugar().Error("Failed to get k8s service account", zap.Error(err))
		return nil, fmt.Errorf("getting k8s service account failed with: %w", err)
	}
	return kServiceAccount, nil
}

func (i *Identity) DeleteWorkloadIdentity(ctx context.Context, projectID, namespace string, identifiable duck.Identifiable) error {
	kServiceAccountName := resources.GenerateServiceAccountName(identifiable.GetIdentity())
	kServiceAccount, err := i.KubeClient.CoreV1().ServiceAccounts(namespace).Get(kServiceAccountName, metav1.GetOptions{})
	if err != nil {
		// k8s ServiceAccount should be there.
		return fmt.Errorf("getting k8s service account failed with: %s", err)
	}
	if kServiceAccount != nil && len(kServiceAccount.OwnerReferences) == 1 {
		logging.FromContext(ctx).Desugar().Debug("Removing iam policy binding.")
		if err := resources.RemoveIamPolicyBinding(ctx, projectID, identifiable.GetIdentity(), kServiceAccount); err != nil {
			return fmt.Errorf("removing iam policy binding failed with: %s", err)
		}
	}
	return nil
}
