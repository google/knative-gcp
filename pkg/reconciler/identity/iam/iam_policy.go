/*
Copyright 2020 Google LLC.

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

package iam

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/iam/admin/apiv1"
	gclient "github.com/google/knative-gcp/pkg/gclient/iam/admin"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type action = int

const (
	actionAdd action = iota
	actionRemove
)

type GServiceAccount string
type RoleName iam.RoleName

type modificationRequest struct {
	serviceAccount GServiceAccount
	role           iam.RoleName
	member         string
	action         action
	respCh         chan error
}

type roleModification struct {
	addMembers    sets.String
	removeMembers sets.String
}

type batchedModifications struct {
	roleModifications map[iam.RoleName]*roleModification
	listeners         []chan<- error
}

type getPolicyResponse struct {
	account GServiceAccount
	policy  *iam.Policy
	err     error
}

type setPolicyResponse struct {
}

// IAMPolicyManager is an interface for making changes to a Google service account's IAM policy.
type IAMPolicyManager interface {
	AddIAMPolicyBinding(ctx context.Context, account GServiceAccount, member string, role RoleName) error
	RemoveIAMPolicyBinding(ctx context.Context, account GServiceAccount, member string, role RoleName) error
}

var (
	globalManager       IAMPolicyManager
	createGlobalManager sync.Once
)

// DefaultIAMPolicyManager returns a shared global policy manager.
func DefaultIAMPolicyManager() IAMPolicyManager {
	createGlobalManager.Do(func() {
		c, err := admin.NewIamClient(context.Background())
		if err != nil {
			panic(err)
		}
		m, err := NewIAMPolicyManager(context.Background(), c)
		if err != nil {
			panic(err)
		}
		globalManager = m
	})
	return globalManager
}

// manager is an IAMPolicyManager which serializes and batches IAM policy changes to a Google
// Service Account to avoid conflicting changes.
type manager struct {
	iam         gclient.IamClient
	requestCh   chan *modificationRequest
	pending     map[GServiceAccount]*batchedModifications // a non-nil batch indicates an outstanding request
	getPolicyCh chan *getPolicyResponse
}

// NewIAMPolicyManager creates an IAMPolicyManager using the given IamClient. The IAMPolicyManager
// will execute until ctx is cancelled.
func NewIAMPolicyManager(ctx context.Context, client gclient.IamClient) (IAMPolicyManager, error) {
	m := &manager{
		iam:         client,
		requestCh:   make(chan *modificationRequest),
		pending:     make(map[GServiceAccount]*batchedModifications),
		getPolicyCh: make(chan *getPolicyResponse),
	}
	go m.manage(ctx)
	return m, nil
}

// AddIAMPolicyBinding adds or updates an IAM policy binding for the given account and role to
// include member. This call will block until the IAM update succeeds or fails or until ctx is
// cancelled.
func (m *manager) AddIAMPolicyBinding(ctx context.Context, account GServiceAccount, member string, role RoleName) error {
	return m.doRequest(ctx, &modificationRequest{
		serviceAccount: account,
		role:           iam.RoleName(role),
		member:         member,
		action:         actionAdd,
		respCh:         make(chan error, 1),
	})
}

// RemoveIAMPolicyBinding removes or updates an IAM policy binding for the given account and role to
// remove member. This call will block until the IAM update succeeds or fails or until ctx is
// cancelled.
func (m *manager) RemoveIAMPolicyBinding(ctx context.Context, account GServiceAccount, member string, role RoleName) error {
	return m.doRequest(ctx, &modificationRequest{
		serviceAccount: account,
		role:           iam.RoleName(role),
		member:         member,
		action:         actionRemove,
		respCh:         make(chan error, 1),
	})
}

func (m *manager) doRequest(ctx context.Context, req *modificationRequest) error {
	select {
	case m.requestCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-req.respCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// manage serializes IAM updates by batching updates for each service account in m.pending and
// applying those updates once the service account's policy has been retrieved. manage maintains the
// invariant that only one set or get request can be outstanding for a given service account by
// starting a request whenever a batch is added to m.pending and by removing a batch from m.pending
// whenever a response is received.
//
// manage receives requests on m.requestCh and adds their modifications to
// the service account's modification batch in m.pending. When a new batch is created, manage will
// initiate a call to GetIAMPolicy which will return its result on m.getPolicyCh. When manage
// receives a policy on getPolicyCh it will apply all batched modifications to that policy and
// initiate a call to SetIAMPolicy which will also return its result m.getPolicyCh. When there are
// no batched modifications to apply to a policy, manage will instead discard the policy and delete
// the service account's entry in m.pending.
func (m *manager) manage(ctx context.Context) {
	for {
		select {
		case req := <-m.requestCh:
			if err := m.makeModificationRequest(ctx, req); err != nil {
				req.respCh <- err
			}
		case getPolicy := <-m.getPolicyCh:
			batched := m.pending[getPolicy.account]
			if len(batched.listeners) == 0 {
				delete(m.pending, getPolicy.account)
				break
			}
			if getPolicy.err != nil {
				for _, listener := range batched.listeners {
					listener <- getPolicy.err
				}
				delete(m.pending, getPolicy.account)
				break
			}
			m.pending[getPolicy.account] = &batchedModifications{
				roleModifications: make(map[iam.RoleName]*roleModification),
			}
			go m.applyBatchedModifications(ctx, getPolicy.account, getPolicy.policy, batched)
		case <-ctx.Done():
			for _, batched := range m.pending {
				for _, listener := range batched.listeners {
					listener <- ctx.Err()
				}
			}
			return
		}
	}
}

// makeModificationRequest adds the modification request to the service account's existing batch if
// one exists. Otherwise it will create a new batch and start a call to getPolicy.
func (m *manager) makeModificationRequest(ctx context.Context, req *modificationRequest) error {
	batched := m.pending[req.serviceAccount]
	if batched == nil {
		batched = &batchedModifications{roleModifications: make(map[iam.RoleName]*roleModification)}
		m.pending[req.serviceAccount] = batched
		go m.getPolicy(ctx, req.serviceAccount)
	}

	mod := batched.roleModifications[req.role]
	if mod == nil {
		mod = &roleModification{
			addMembers:    sets.NewString(),
			removeMembers: sets.NewString(),
		}
		batched.roleModifications[req.role] = mod
	}
	switch req.action {
	case actionAdd:
		if mod.removeMembers.Has(req.member) {
			return fmt.Errorf("conflicting remove of member %s", req.member)
		}
		mod.addMembers.Insert(req.member)
	case actionRemove:
		if mod.addMembers.Has(req.member) {
			return fmt.Errorf("conflicting add of member %s", req.member)
		}
		mod.removeMembers.Insert(req.member)
	}
	batched.listeners = append(batched.listeners, req.respCh)
	return nil
}

// getPolicy calls GetIamPolicy for the given service account and puts the result in m.getPolicyCh.
func (m *manager) getPolicy(ctx context.Context, account GServiceAccount) {
	policy, err := m.iam.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: admin.IamServiceAccountPath("-", string(account))})
	select {
	case m.getPolicyCh <- &getPolicyResponse{account: account, policy: policy, err: err}:
	case <-ctx.Done():
	}
}

// applyBatchedModifications applies given set of batched modifications to the IAM policy and calls
// SetIAMPolicy for the given service account placing the result in m.getPolicyCh.
func (m *manager) applyBatchedModifications(ctx context.Context, account GServiceAccount, policy *iam.Policy, batched *batchedModifications) {
	for role, mod := range batched.roleModifications {
		applyRoleModifications(policy, role, mod)
	}
	policy, err := m.iam.SetIamPolicy(ctx, &admin.SetIamPolicyRequest{
		Resource: admin.IamServiceAccountPath("-", string(account)),
		Policy:   policy,
	})
	for _, listener := range batched.listeners {
		listener <- err
	}
	select {
	case m.getPolicyCh <- &getPolicyResponse{account: account, policy: policy, err: err}:
	case <-ctx.Done():
	}
}

func applyRoleModifications(policy *iam.Policy, role iam.RoleName, mod *roleModification) {
	for member := range mod.addMembers {
		policy.Add(member, role)
	}
	for member := range mod.removeMembers {
		policy.Remove(member, role)
	}
}
