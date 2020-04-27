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

// Package identity contains the identity reconciler
package identity

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	duck "github.com/google/knative-gcp/pkg/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	Role                         = "roles/iam.workloadIdentityUser"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

func NewIdentity(ctx context.Context, policyManager iam.IAMPolicyManager) *Identity {
	return &Identity{
		kubeClient:    kubeclient.Get(ctx),
		policyManager: policyManager,
	}
}

type Identity struct {
	kubeClient    kubernetes.Interface
	policyManager iam.IAMPolicyManager
}

// ReconcileWorkloadIdentity will create a k8s service account, add ownerReference to it,
// and add iam policy binding between this k8s service account and its corresponding GCP service account.
func (i *Identity) ReconcileWorkloadIdentity(ctx context.Context, projectID string, identifiable duck.Identifiable) (*corev1.ServiceAccount, error) {
	status := identifiable.IdentityStatus()
	// Remove status.ServiceAccountName from last reconcile circle.
	status.ServiceAccountName = ""
	// Create corresponding k8s ServiceAccount if it doesn't exist.
	namespace := identifiable.GetObjectMeta().GetNamespace()
	clusterName := identifiable.GetObjectMeta().GetAnnotations()[v1alpha1.ClusterNameAnnotation]
	kServiceAccount, err := i.createServiceAccount(ctx, namespace, identifiable.IdentitySpec().GoogleServiceAccount, clusterName)
	if err != nil {
		status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), workloadIdentityFailed, err.Error())
		return nil, fmt.Errorf("failed to get k8s ServiceAccount: %w", err)
	}
	// Add ownerReference to K8s ServiceAccount.
	expectOwnerReference := *kmeta.NewControllerRef(identifiable)
	expectOwnerReference.Controller = ptr.Bool(false)
	if !ownerReferenceExists(kServiceAccount, expectOwnerReference) {
		kServiceAccount.OwnerReferences = append(kServiceAccount.OwnerReferences, expectOwnerReference)
		if _, err := i.kubeClient.CoreV1().ServiceAccounts(kServiceAccount.Namespace).Update(kServiceAccount); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update OwnerReferences", zap.Error(err))
			status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), workloadIdentityFailed, err.Error())
			return nil, fmt.Errorf("failed to update OwnerReferences: %w", err)
		}
	}

	// Add iam policy binding to GCP ServiceAccount.
	if err := i.addIamPolicyBinding(ctx, projectID, identifiable.IdentitySpec().GoogleServiceAccount, kServiceAccount); err != nil {
		status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), workloadIdentityFailed, err.Error())
		return kServiceAccount, fmt.Errorf("adding iam policy binding failed with: %w", err)
	}
	status.ServiceAccountName = kServiceAccount.Name
	status.MarkWorkloadIdentityConfigured(identifiable.ConditionSet())
	return kServiceAccount, nil
}

// DeleteWorkloadIdentity will remove iam policy binding between k8s service account and its corresponding GCP service account,
// if this k8s service account only has one ownerReference.
func (i *Identity) DeleteWorkloadIdentity(ctx context.Context, projectID string, identifiable duck.Identifiable) error {
	status := identifiable.IdentityStatus()
	// If the ServiceAccountName wasn't set in the status, it means there are errors when reconciling workload identity.
	// If ReconcileWorkloadIdentity error is for k8s service account, it will be handled by k8s ownerReferences Garbage collection.
	// If ReconcileWorkloadIdentity error is for add iam policy binding, then no need to remove it.
	// Thus, for this case, we simply return.
	if status.ServiceAccountName == "" {
		return nil
	}
	namespace := identifiable.GetObjectMeta().GetNamespace()
	clusterName := identifiable.GetObjectMeta().GetAnnotations()[v1alpha1.ClusterNameAnnotation]
	kServiceAccountName := resources.GenerateServiceAccountName(identifiable.IdentitySpec().GoogleServiceAccount, clusterName)
	kServiceAccount, err := i.kubeClient.CoreV1().ServiceAccounts(namespace).Get(kServiceAccountName, metav1.GetOptions{})
	if err != nil {
		status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), deleteWorkloadIdentityFailed, err.Error())
		// k8s ServiceAccount should be there.
		return fmt.Errorf("getting k8s service account failed with: %w", err)
	}
	if kServiceAccount != nil && len(kServiceAccount.OwnerReferences) == 1 {
		logging.FromContext(ctx).Desugar().Debug("Removing iam policy binding.")
		if err := i.removeIamPolicyBinding(ctx, projectID, identifiable.IdentitySpec().GoogleServiceAccount, kServiceAccount); err != nil {
			status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), deleteWorkloadIdentityFailed, err.Error())
			return fmt.Errorf("removing iam policy binding failed with: %w", err)
		}
	}
	return nil
}

func (i *Identity) createServiceAccount(ctx context.Context, namespace, gServiceAccount, clusterName string) (*corev1.ServiceAccount, error) {
	kServiceAccountName := resources.GenerateServiceAccountName(gServiceAccount, clusterName)
	kServiceAccount, err := i.kubeClient.CoreV1().ServiceAccounts(namespace).Get(kServiceAccountName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			expect := resources.MakeServiceAccount(namespace, gServiceAccount, clusterName)
			logging.FromContext(ctx).Desugar().Debug("Creating k8s service account", zap.Any("ksa", expect))
			kServiceAccount, err := i.kubeClient.CoreV1().ServiceAccounts(expect.Namespace).Create(expect)
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

// TODO he iam policy binding should be mocked so that we can unit test it. issue https://github.com/google/knative-gcp/issues/657
// addIamPolicyBinding will add iam policy binding, which is related to a provided k8s ServiceAccount, to a GCP ServiceAccount.
func (i *Identity) addIamPolicyBinding(ctx context.Context, projectID, gServiceAccount string, kServiceAccount *corev1.ServiceAccount) error {
	projectID, err := utils.ProjectID(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	// currentMember will end up as "serviceAccount:projectId.svc.id.goog[k8s-namespace/ksa-name]".
	currentMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", projectID, kServiceAccount.Namespace, kServiceAccount.Name)

	return i.policyManager.AddIAMPolicyBinding(ctx, iam.GServiceAccount(gServiceAccount), currentMember, Role)
}

// removeIamPolicyBinding will remove iam policy binding, which is related to a provided k8s ServiceAccount, from a GCP ServiceAccount.
func (i *Identity) removeIamPolicyBinding(ctx context.Context, projectID, gServiceAccount string, kServiceAccount *corev1.ServiceAccount) error {
	projectID, err := utils.ProjectID(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	// currentMember will end up as "serviceAccount:projectId.svc.id.goog[k8s-namespace/ksa-name]".
	currentMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", projectID, kServiceAccount.Namespace, kServiceAccount.Name)

	return i.policyManager.RemoveIAMPolicyBinding(ctx, iam.GServiceAccount(gServiceAccount), currentMember, Role)
}

// ownerReferenceExists checks if a K8s ServiceAccount contains specific ownerReference
func ownerReferenceExists(kServiceAccount *corev1.ServiceAccount, expect metav1.OwnerReference) bool {
	references := kServiceAccount.OwnerReferences
	for _, reference := range references {
		if reference.Name == expect.Name {
			return true
		}
	}
	return false
}
