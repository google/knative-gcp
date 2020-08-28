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
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	duck "github.com/google/knative-gcp/pkg/duck/v1"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	Role                         = "roles/iam.workloadIdentityUser"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

func NewIdentity(ctx context.Context, policyManager iam.IAMPolicyManager, gcpAuthStore *gcpauth.Store) *Identity {
	return &Identity{
		kubeClient:    kubeclient.Get(ctx),
		policyManager: policyManager,
		gcpAuthStore:  gcpAuthStore,
	}
}

func NewGCPAuthStore(ctx context.Context, cmw configmap.Watcher) *gcpauth.Store {
	gcpAuthStore := gcpauth.NewStore(logging.FromContext(ctx).Named("config-gcp-auth-store"))
	gcpAuthStore.WatchConfigs(cmw)
	return gcpAuthStore
}

type Identity struct {
	kubeClient    kubernetes.Interface
	policyManager iam.IAMPolicyManager
	gcpAuthStore  *gcpauth.Store
}

// ReconcileWorkloadIdentity will create a k8s service account, add ownerReference to it,
// and add iam policy binding between this k8s service account and its corresponding GCP service account.
func (i *Identity) ReconcileWorkloadIdentity(ctx context.Context, projectID string, identifiable duck.Identifiable) (*corev1.ServiceAccount, error) {
	status := identifiable.IdentityStatus()
	// Create corresponding k8s ServiceAccount if it doesn't exist.

	identityNames, err := i.getGoogleServiceAccountName(ctx, identifiable)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("failed to get Google service account name", zap.Error(err))
		status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), workloadIdentityFailed, err.Error())
		return nil, fmt.Errorf(`failed to get Google service account name: %w`, err)
	} else if identityNames.GoogleServiceAccountName == "" {
		// If there is no Google service account paired with current Kubernetes service account in GCP auth configmap, no further reconciliation.
		return nil, nil
	}

	kServiceAccount, err := i.createServiceAccount(ctx, identityNames)
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
	if err := i.addIamPolicyBinding(ctx, projectID, identityNames); err != nil {
		status.MarkWorkloadIdentityFailed(identifiable.ConditionSet(), workloadIdentityFailed, err.Error())
		return kServiceAccount, fmt.Errorf("adding iam policy binding failed with: %w", err)
	}
	status.MarkWorkloadIdentityReady(identifiable.ConditionSet())
	return kServiceAccount, nil
}

// DeleteWorkloadIdentity will remove iam policy binding between k8s service account and its corresponding GCP service account,
// if this k8s service account only has one ownerReference.
func (i *Identity) DeleteWorkloadIdentity(ctx context.Context, projectID string, identifiable duck.Identifiable) error {
	status := identifiable.IdentityStatus()

	identityNames, err := i.getGoogleServiceAccountName(ctx, identifiable)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("failed to get Google service account name", zap.Error(err))
		status.MarkWorkloadIdentityUnknown(identifiable.ConditionSet(), deleteWorkloadIdentityFailed, err.Error())
		return fmt.Errorf(`failed to get Google service account name: %w`, err)
	} else if identityNames.GoogleServiceAccountName == "" {
		// If there is no Google service account paired with current Kubernetes service account in GCP auth configmap, no further reconciliation.
		return nil
	}

	kServiceAccount, err := i.kubeClient.CoreV1().ServiceAccounts(identityNames.Namespace).Get(identityNames.KServiceAccountName, metav1.GetOptions{})
	if err != nil {
		status.MarkWorkloadIdentityUnknown(identifiable.ConditionSet(), deleteWorkloadIdentityFailed, err.Error())
		// k8s ServiceAccount should be there.
		return fmt.Errorf("getting k8s service account failed with: %w", err)
	}
	if kServiceAccount != nil && len(kServiceAccount.OwnerReferences) == 1 {
		logging.FromContext(ctx).Desugar().Debug("Removing iam policy binding.")
		if err := i.removeIamPolicyBinding(ctx, projectID, identityNames); err != nil {
			status.MarkWorkloadIdentityUnknown(identifiable.ConditionSet(), deleteWorkloadIdentityFailed, err.Error())
			return fmt.Errorf("removing iam policy binding failed with: %w", err)
		}
	}
	return nil
}

// getGoogleServiceAccountName will return Google service account name and corresponding raw Kubernetes service account name.
func (i *Identity) getGoogleServiceAccountName(ctx context.Context, identifiable duck.Identifiable) (resources.IdentityNames, error) {
	namespace := identifiable.GetObjectMeta().GetNamespace()
	ad := i.gcpAuthStore.Load()
	if ad == nil || ad.GCPAuthDefaults == nil {
		logging.FromContext(ctx).Desugar().Error("Failed to get default config from GCP auth configmap")
		return resources.IdentityNames{}, fmt.Errorf("failed to get default config from GCP auth configmap")
	}
	return resources.IdentityNames{
		KServiceAccountName:      identifiable.IdentitySpec().ServiceAccountName,
		GoogleServiceAccountName: ad.GCPAuthDefaults.WorkloadIdentityGSA(namespace, identifiable.IdentitySpec().ServiceAccountName),
		Namespace:                namespace,
	}, nil
}

func (i *Identity) createServiceAccount(ctx context.Context, identityNames resources.IdentityNames) (*corev1.ServiceAccount, error) {
	kServiceAccount, err := i.kubeClient.CoreV1().ServiceAccounts(identityNames.Namespace).Get(identityNames.KServiceAccountName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			expect := resources.MakeServiceAccount(identityNames)
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
func (i *Identity) addIamPolicyBinding(ctx context.Context, projectID string, identityNames resources.IdentityNames) error {
	projectID, err := utils.ProjectID(projectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	// currentMember will end up as "serviceAccount:projectId.svc.id.goog[k8s-namespace/ksa-name]".
	currentMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", projectID, identityNames.Namespace, identityNames.KServiceAccountName)

	return i.policyManager.AddIAMPolicyBinding(ctx, iam.GServiceAccount(identityNames.GoogleServiceAccountName), currentMember, Role)
}

// removeIamPolicyBinding will remove iam policy binding, which is related to a provided k8s ServiceAccount, from a GCP ServiceAccount.
func (i *Identity) removeIamPolicyBinding(ctx context.Context, projectID string, identityNames resources.IdentityNames) error {
	projectID, err := utils.ProjectID(projectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	// currentMember will end up as "serviceAccount:projectId.svc.id.goog[k8s-namespace/ksa-name]".
	currentMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", projectID, identityNames.Namespace, identityNames.KServiceAccountName)

	return i.policyManager.RemoveIAMPolicyBinding(ctx, iam.GServiceAccount(identityNames.GoogleServiceAccountName), currentMember, Role)
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
