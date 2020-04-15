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

package admin

import (
	"context"

	"cloud.google.com/go/iam"
	admin "cloud.google.com/go/iam/admin/apiv1"
	"github.com/golang/protobuf/proto"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type client struct {
	policies map[string]*iam.Policy
}

func NewTestClient() IamClient {
	return client{
		policies: make(map[string]*iam.Policy),
	}
}

func (c client) GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iam.Policy, error) {
	if policy := c.policies[req.Resource]; policy == nil {
		return nil, status.Error(codes.NotFound, "service account not found")
	} else {
		return &iam.Policy{InternalProto: proto.Clone(policy.InternalProto).(*iampb.Policy)}, nil
	}
}

func (c client) SetIamPolicy(ctx context.Context, req *admin.SetIamPolicyRequest) (*iam.Policy, error) {
	c.policies[req.Resource] = &iam.Policy{InternalProto: proto.Clone(req.Policy.InternalProto).(*iampb.Policy)}
	return &iam.Policy{InternalProto: proto.Clone(c.policies[req.Resource].InternalProto).(*iampb.Policy)}, nil
}
