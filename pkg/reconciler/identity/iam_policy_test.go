/*
Copyright 2019 Google LLC.

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

package identity_test

import (
	"context"
	"testing"

	"cloud.google.com/go/iam"
	admin "cloud.google.com/go/iam/admin/apiv1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	gclient "github.com/google/knative-gcp/pkg/gclient/iam/admin"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testProject  = "project"
	testAccount  = "account"
	testResource = "projects/project/serviceAccounts/account"
	member1      = "member1"
	member2      = "member2"
	member3      = "member3"
	role1        = iam.RoleName("roles/role1")
	role2        = iam.RoleName("roles/role2")
)

func makePolicy(roles map[iam.RoleName][]string) *iam.Policy {
	policy := new(iam.Policy)
	for role, members := range roles {
		for _, member := range members {
			policy.Add(member, role)
		}
	}
	return policy
}

func TestAddPolicyBinding(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name          string
		initialPolicy *iam.Policy
		role          iam.RoleName
		member        string
		wantErrCode   codes.Code
		wantMembers   map[iam.RoleName][]string
	}{{
		name:        "not found",
		role:        role1,
		member:      member1,
		wantErrCode: codes.NotFound,
	}, {
		name:          "add to empty policy",
		initialPolicy: &iam.Policy{},
		role:          role1,
		member:        member1,
		wantMembers: map[iam.RoleName][]string{
			role1: {member1},
		},
	}, {
		name: "add to new role",
		initialPolicy: makePolicy(map[iam.RoleName][]string{
			role1: {member1},
		}),
		role:   role2,
		member: member2,
		wantMembers: map[iam.RoleName][]string{
			role1: {member1},
			role2: {member2},
		},
	}, {
		name: "add existing binding",
		initialPolicy: makePolicy(map[iam.RoleName][]string{
			role1: {member1},
		}),
		role:   role1,
		member: member1,
		wantMembers: map[iam.RoleName][]string{
			role1: {member1},
		},
	}, {
		name: "add to existing role",
		initialPolicy: makePolicy(map[iam.RoleName][]string{
			role1: {member1},
		}),
		role:   role1,
		member: member2,
		wantMembers: map[iam.RoleName][]string{
			role1: {member1, member2},
		},
	}}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := gclient.NewTestClient()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m, err := identity.NewIAMPolicyManager(ctx, client)
			if err != nil {
				t.Fatal(err)
			}
			if tc.initialPolicy != nil {
				_, err = client.SetIamPolicy(ctx, &admin.SetIamPolicyRequest{Resource: testResource, Policy: tc.initialPolicy})
				if err != nil {
					t.Fatal(err)
				}
			}

			err = m.AddIAMPolicyBinding(ctx, identity.GServiceAccount{ProjectID: testProject, Name: testAccount}, tc.member, tc.role)
			if code := status.Code(err); tc.wantErrCode != code {
				t.Fatalf("error code: want %v, got %v", tc.wantErrCode, code)
			}
			if tc.wantErrCode != codes.OK {
				return
			}

			policy, err := client.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: testResource})
			if err != nil {
				t.Fatal(err)
			}
			for role, members := range tc.wantMembers {
				if diff := cmp.Diff(members, policy.Members(role), cmpopts.SortSlices(func(m1, m2 string) bool {
					return m1 < m2
				})); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
				}
			}
		})
	}
}

func TestRemovePolicyBinding(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name          string
		initialPolicy *iam.Policy
		role          iam.RoleName
		member        string
		wantErrCode   codes.Code
		wantMembers   map[iam.RoleName][]string
	}{{
		name:        "not found",
		role:        role1,
		member:      member1,
		wantErrCode: codes.NotFound,
	}, {
		name:          "remove from empty policy",
		initialPolicy: &iam.Policy{},
		role:          role1,
		member:        member1,
		wantMembers: map[iam.RoleName][]string{
			role1: nil,
		},
	}, {
		name: "remove from new role",
		initialPolicy: makePolicy(map[iam.RoleName][]string{
			role1: {member1},
		}),
		role:   role2,
		member: member2,
		wantMembers: map[iam.RoleName][]string{
			role1: {member1},
		},
	}, {
		name: "remove binding",
		initialPolicy: makePolicy(map[iam.RoleName][]string{
			role1: {member1},
		}),
		role:   role1,
		member: member1,
		wantMembers: map[iam.RoleName][]string{
			role1: nil,
		},
	}, {
		name: "remove from existing role",
		initialPolicy: makePolicy(map[iam.RoleName][]string{
			role1: {member1, member2},
		}),
		role:   role1,
		member: member2,
		wantMembers: map[iam.RoleName][]string{
			role1: {member1},
		},
	}}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := gclient.NewTestClient()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m, err := identity.NewIAMPolicyManager(ctx, client)
			if err != nil {
				t.Fatal(err)
			}
			if tc.initialPolicy != nil {
				_, err = client.SetIamPolicy(ctx, &admin.SetIamPolicyRequest{Resource: testResource, Policy: tc.initialPolicy})
				if err != nil {
					t.Fatal(err)
				}
			}

			err = m.RemoveIAMPolicyBinding(ctx, identity.GServiceAccount{ProjectID: testProject, Name: testAccount}, tc.member, tc.role)
			if code := status.Code(err); tc.wantErrCode != code {
				t.Fatalf("error code: want %v, got %v", tc.wantErrCode, code)
			}
			if tc.wantErrCode != codes.OK {
				return
			}

			policy, err := client.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: testResource})
			if err != nil {
				t.Fatal(err)
			}
			for role, members := range tc.wantMembers {
				if diff := cmp.Diff(members, policy.Members(role), cmpopts.SortSlices(func(m1, m2 string) bool {
					return m1 < m2
				})); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
				}
			}
		})
	}
}
