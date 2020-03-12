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
	"context"
	"fmt"

	"github.com/google/knative-gcp/pkg/utils"
	"google.golang.org/api/iam/v1"
	corev1 "k8s.io/api/core/v1"
)

// AddIamPolicyBinding will add iam policy binding, which is related to a provided k8s ServiceAccount, to a GCP ServiceAccount.
func AddIamPolicyBinding(ctx context.Context, projectID string, gServiceAccount *string, kServiceAccount *corev1.ServiceAccount) error {
	if err := SetIamPolicy(ctx, "add", projectID, gServiceAccount, kServiceAccount); err != nil {
		return fmt.Errorf("failed to add iam policy binding: %w", err)
	}
	return nil
}

// RemoveIamPolicyBinding will remove iam policy binding, which is related to a provided k8s ServiceAccount, from a GCP ServiceAccount.
func RemoveIamPolicyBinding(ctx context.Context, projectID string, gServiceAccount *string, kServiceAccount *corev1.ServiceAccount) error {
	if err := SetIamPolicy(ctx, "remove", projectID, gServiceAccount, kServiceAccount); err != nil {
		return fmt.Errorf("failed to remove iam policy binding: %w", err)
	}
	return nil
}

func SetIamPolicy(ctx context.Context, action, projectID string, gServiceAccount *string, kServiceAccount *corev1.ServiceAccount) error {
	iamService, err := iam.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to set google iam service: %w", err)
	}

	projectId, err := utils.ProjectID(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	resource := "projects/" + projectId + "/serviceAccounts/" + *gServiceAccount
	resp, err := iamService.Projects.ServiceAccounts.GetIamPolicy(resource).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get iam policy: %w", err)
	}

	currentMember := "serviceAccount:" + projectId + ".svc.id.goog[" + kServiceAccount.Namespace + "/" + kServiceAccount.Name + "]"
	rb := MakeSetIamPolicyRequest(resp.Bindings, action, currentMember)

	if _, err := iamService.Projects.ServiceAccounts.SetIamPolicy(resource, rb).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to set iam policy: %w", err)
	}

	return nil
}

func MakeSetIamPolicyRequest(bindings []*iam.Binding, action, currentMember string) *iam.SetIamPolicyRequest {
	switch action {
	case "add":
		return &iam.SetIamPolicyRequest{
			Policy: &iam.Policy{
				Bindings: append(bindings, &iam.Binding{
					Members: []string{currentMember},
					Role:    "roles/iam.workloadIdentityUser"}),
			},
		}
	case "remove":
		for _, binding := range bindings {
			if binding.Role == "roles/iam.workloadIdentityUser" {
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
