package identity

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/iam/admin/apiv1"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
)

type action = int

const (
	Add = iota
	Remove
)

type GServiceAccount struct {
	ProjectID string
	Name      string
}

type modificationRequest struct {
	serviceAccount GServiceAccount
	role           iam.RoleName
	member         string
	action         action
	respCh         chan error
}

type roleModification struct {
	addMembers    map[string]bool
	removeMembers map[string]bool
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

type IAMPolicyManager struct {
	iam         *admin.IamClient
	requestCh   chan *modificationRequest
	pending     map[GServiceAccount]*batchedModifications
	getPolicyCh chan *getPolicyResponse
}

func NewIAMPolicyManager(ctx context.Context) (*IAMPolicyManager, error) {
	svc, err := admin.NewIamClient(ctx)
	if err != nil {
		return nil, err
	}
	m := &IAMPolicyManager{
		iam:         svc,
		requestCh:   make(chan *modificationRequest),
		pending:     make(map[GServiceAccount]*batchedModifications),
		getPolicyCh: make(chan *getPolicyResponse),
	}
	go m.manage(ctx)
	return m, nil
}

func (m *IAMPolicyManager) AddIAMPolicyBinding(ctx context.Context, account GServiceAccount, member string, role iam.RoleName) error {
	return m.doRequest(ctx, &modificationRequest{
		serviceAccount: account,
		role:           role,
		member:         member,
		action:         Add,
		respCh:         make(chan error),
	})
}

func (m *IAMPolicyManager) RemoveIAMPolicyBinding(ctx context.Context, account GServiceAccount, member string, role iam.RoleName) error {
	return m.doRequest(ctx, &modificationRequest{
		serviceAccount: account,
		role:           role,
		member:         member,
		action:         Remove,
		respCh:         make(chan error),
	})
}

func (m *IAMPolicyManager) doRequest(ctx context.Context, req *modificationRequest) error {
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

func (m *IAMPolicyManager) manage(ctx context.Context) {
	for {
		select {
		case req := <-m.requestCh:
			if err := m.makeModificationRequest(ctx, req); err != nil {
				go sendResponse(ctx, req.respCh, err)
			}
		case getPolicy := <-m.getPolicyCh:
			batched := m.pending[getPolicy.account]
			if len(batched.listeners) == 0 {
				delete(m.pending, getPolicy.account)
				break
			}
			if getPolicy.err != nil {
				for _, listener := range batched.listeners {
					go sendResponse(ctx, listener, getPolicy.err)
				}
				delete(m.pending, getPolicy.account)
			}
			m.pending[getPolicy.account] = &batchedModifications{
				roleModifications: make(map[iam.RoleName]*roleModification),
			}
			go m.applyBatchedModifications(ctx, getPolicy.account, getPolicy.policy, batched)
		case <-ctx.Done():
			for _, batched := range m.pending {
				for _, listener := range batched.listeners {
					select {
					case listener <- ctx.Err():
					default:
					}
				}
			}
			return
		}
	}
}

func (m *IAMPolicyManager) makeModificationRequest(ctx context.Context, req *modificationRequest) error {
	batched := m.pending[req.serviceAccount]
	mod := batched.roleModifications[req.role]
	if mod == nil {
		mod = &roleModification{
			addMembers:    make(map[string]bool),
			removeMembers: make(map[string]bool),
		}
		if batched != nil {
			batched.roleModifications[req.role] = mod
		}
	}
	switch req.action {
	case Add:
		if mod.removeMembers[req.member] {
			return fmt.Errorf("conflicting remove of member %s", req.member)
		}
		mod.addMembers[req.member] = true
	case Remove:
		if mod.addMembers[req.member] {
			return fmt.Errorf("conflicting add of member %s", req.member)
		}
		mod.removeMembers[req.member] = true
	}
	if batched == nil {
		batched = &batchedModifications{
			roleModifications: map[iam.RoleName]*roleModification{
				req.role: mod,
			},
		}
		m.pending[req.serviceAccount] = batched
		go m.getPolicy(ctx, req.serviceAccount)
	}
	batched.listeners = append(batched.listeners, req.respCh)
	return nil
}

func (m *IAMPolicyManager) getPolicy(ctx context.Context, account GServiceAccount) {
	policy, err := m.iam.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: admin.IamServiceAccountPath(account.ProjectID, account.Name)})
	//Projects.ServiceAccounts.GetIamPolicy(resource).Context(ctx).Do()
	select {
	case m.getPolicyCh <- &getPolicyResponse{account: account, policy: policy, err: err}:
	case <-ctx.Done():
	}
}

func (m *IAMPolicyManager) applyBatchedModifications(ctx context.Context, account GServiceAccount, policy *iam.Policy, batched *batchedModifications) {
	for role, mod := range batched.roleModifications {
		applyRoleModifications(policy, role, mod)
	}
	policy, err := m.iam.SetIamPolicy(ctx, &admin.SetIamPolicyRequest{
		Resource: admin.IamServiceAccountPath(account.ProjectID, account.Name),
		Policy:   policy,
	})
	for _, listener := range batched.listeners {
		go sendResponse(ctx, listener, err)
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

func sendResponse(ctx context.Context, respCh chan<- error, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	select {
	case respCh <- err:
	case <-ctx.Done():
	}
}
