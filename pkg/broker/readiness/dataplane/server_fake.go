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

// Package authcheck provides utilities to check authentication configuration for data plane resources.
// probechecker.go  utilities to perform a probe check for liviness and readiness.
package dataplane

import (
	context "context"

	"github.com/google/knative-gcp/pkg/broker/config"
	. "github.com/google/knative-gcp/pkg/broker/readiness"
)

type FakeConfigReadinessCheckServer struct {
}

func (f *FakeConfigReadinessCheckServer) Start() {
	return
}

func (f *FakeConfigReadinessCheckServer) GetCellTenantGeneration(ctx context.Context, req *GetGenerationReq) (*GetGenerationResp, error) {
	return &GetGenerationResp{}, nil
}

func (f *FakeConfigReadinessCheckServer) GetTargetGeneration(ctx context.Context, req *GetGenerationReq) (*GetGenerationResp, error) {
	return &GetGenerationResp{}, nil
}

func (f *FakeConfigReadinessCheckServer) UpdateCellTenantGeneration(b *config.CellTenant) {

}

func (f *FakeConfigReadinessCheckServer) UpdateTargetGeneration(t *config.Target) {

}

func (f *FakeConfigReadinessCheckServer) CleanupStaleRecords(targets config.ReadonlyTargets) {

}
