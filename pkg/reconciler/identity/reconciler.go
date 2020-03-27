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
	"strings"

	"go.uber.org/zap"
	"google.golang.org/api/iam/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/google/knative-gcp/pkg/duck"
	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	Add              = "add"
	Remove           = "remove"
	Role             = "roles/iam.workloadIdentityUser"
	ConcurrencyError = "googleapi: Error 409: There were concurrent policy changes."
)

func NewIdentity(ctx context.Context) *Identity {
	return &Identity{
		KubeClient: kubeclient.Get(ctx),
	}
}

type Identity struct {
	KubeClient kubernetes.Interface
}

// ReconcileWorkloadIdentity will create a k8s service account, add ownerReference to it,
// and add iam policy binding between this k8s service account and its corresponding GCP service account.
func (i *Identity) ReconcileWorkloadIdentity(ctx context.Context, projectID string, identifiable duck.Identifiable) (*corev1.ServiceAccount, error) {
	// Create corresponding k8s ServiceAccount if it doesn't exist.
	namespace := identifiable.GetObjectMeta().GetNamespace()
	kServiceAccount, err := i.createServiceAccount(ctx, namespace, identifiable.GetIdentity())
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s ServiceAccount: %w", err)
	}
	// Add ownerReference to K8s ServiceAccount.
	expectOwnerReference := *kmeta.NewControllerRef(identifiable)
	expectOwnerReference.Controller = ptr.Bool(false)
	if !ownerReferenceExists(kServiceAccount, expectOwnerReference) {
		kServiceAccount.OwnerReferences = append(kServiceAccount.OwnerReferences, expectOwnerReference)
		if _, err := i.KubeClient.CoreV1().ServiceAccounts(kServiceAccount.Namespace).Update(kServiceAccount); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update OwnerReferences", zap.Error(err))
			return nil, fmt.Errorf("failed to update OwnerReferences: %w", err)
		}
	}

	// Add iam policy binding to GCP ServiceAccount.
	if err := addIamPolicyBinding(ctx, projectID, identifiable.GetIdentity(), kServiceAccount); err != nil {
		return kServiceAccount, fmt.Errorf("adding iam policy binding failed with: %s", err)
	}
	return kServiceAccount, nil
}

// DeleteWorkloadIdentity will remove iam policy binding between k8s service account and its corresponding GCP service account,
// if this k8s service account only has one ownerReference.
func (i *Identity) DeleteWorkloadIdentity(ctx context.Context, projectID string, identifiable duck.Identifiable) error {
	namespace := identifiable.GetObjectMeta().GetNamespace()
	kServiceAccountName := resources.GenerateServiceAccountName(identifiable.GetIdentity())
	kServiceAccount, err := i.KubeClient.CoreV1().ServiceAccounts(namespace).Get(kServiceAccountName, metav1.GetOptions{})
	if err != nil {
		// k8s ServiceAccount should be there.
		return fmt.Errorf("getting k8s service account failed with: %w", err)
	}
	if kServiceAccount != nil && len(kServiceAccount.OwnerReferences) == 1 {
		logging.FromContext(ctx).Desugar().Debug("Removing iam policy binding.")
		if err := removeIamPolicyBinding(ctx, projectID, identifiable.GetIdentity(), kServiceAccount); err != nil {
			return fmt.Errorf("removing iam policy binding failed with: %w", err)
		}
	}
	return nil
}

func (i *Identity) createServiceAccount(ctx context.Context, namespace, gServiceAccount string) (*corev1.ServiceAccount, error) {
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

// TODO he iam policy binding should be mocked so that we can unit test it. issue https://github.com/google/knative-gcp/issues/657
// addIamPolicyBinding will add iam policy binding, which is related to a provided k8s ServiceAccount, to a GCP ServiceAccount.
func addIamPolicyBinding(ctx context.Context, projectID, gServiceAccount string, kServiceAccount *corev1.ServiceAccount) error {
	if err := setIamPolicy(ctx, Add, projectID, gServiceAccount, kServiceAccount); err != nil {
		return err
	}
	return nil
}

// removeIamPolicyBinding will remove iam policy binding, which is related to a provided k8s ServiceAccount, from a GCP ServiceAccount.
func removeIamPolicyBinding(ctx context.Context, projectID, gServiceAccount string, kServiceAccount *corev1.ServiceAccount) error {
	if err := setIamPolicy(ctx, Remove, projectID, gServiceAccount, kServiceAccount); err != nil {
		return err
	}
	return nil
}

func setIamPolicy(ctx context.Context, action, projectID string, gServiceAccount string, kServiceAccount *corev1.ServiceAccount) error {
	iamService, err := iam.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to set google iam service: %w", err)
	}

	projectId, err := utils.ProjectID(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	resource := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, gServiceAccount)
	resp, err := iamService.Projects.ServiceAccounts.GetIamPolicy(resource).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get iam policy: %w", err)
	}

	// currentMember will end up as "serviceAccount:projectId.svc.id.goog[k8s-namespace/ksa-name]".
	currentMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", projectId, kServiceAccount.Namespace, kServiceAccount.Name)
	rb := makeSetIamPolicyRequest(resp.Bindings, action, currentMember)

	err = fmt.Errorf(ConcurrencyError)
	// If the setIamPolicy error is caused by concurrency, retry it.
	for err != nil && strings.Contains(err.Error(), ConcurrencyError) {
		_, err = iamService.Projects.ServiceAccounts.SetIamPolicy(resource, rb).Context(ctx).Do()
	}
	if err != nil {
		return fmt.Errorf("failed to set iam policy: %w", err)
	}

	return nil
}

func makeSetIamPolicyRequest(bindings []*iam.Binding, action, currentMember string) *iam.SetIamPolicyRequest {
	switch action {
	case Add:
		return &iam.SetIamPolicyRequest{
			Policy: &iam.Policy{
				Bindings: append(bindings, &iam.Binding{
					Members: []string{currentMember},
					Role:    Role}),
			},
		}
	case Remove:
		for _, binding := range bindings {
			if binding.Role == Role {
				newMembers := []string{}
				for _, member := range binding.Members {
					if member != currentMember {
						newMembers = append(newMembers, member)
					}
				}
				binding.Members = newMembers
			}
		}
		return &iam.SetIamPolicyRequest{
			Policy: &iam.Policy{
				Bindings: bindings,
			},
		}
	}
	return nil
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
