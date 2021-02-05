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
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/google/knative-gcp/pkg/broker/config"
	. "github.com/google/knative-gcp/pkg/broker/readiness"
	grpc "google.golang.org/grpc"
)

const (
	Port int = 3456
)

type rwMap struct {
	sync.RWMutex
	m map[string]int64
}

type ConfigReadinessCheckServer interface {
	Start()
	GetCellTenantGeneration(ctx context.Context, req *GetGenerationReq) (*GetGenerationResp, error)
	GetTargetGeneration(ctx context.Context, req *GetGenerationReq) (*GetGenerationResp, error)
	UpdateCellTenantGeneration(b *config.CellTenant)
	UpdateTargetGeneration(t *config.Target)
	CleanupStaleRecords(targets config.ReadonlyTargets)
}

type configReadinessCheckServer struct { // TODO  change the name to ConfigCheckServer
	UnimplementedGenerationQueryServiceServer
	cellTenantGenMap *rwMap
	targetGenMap     *rwMap
}

func NewServer() ConfigReadinessCheckServer {
	return &configReadinessCheckServer{
		cellTenantGenMap: &rwMap{m: make(map[CellTenantKeyType]int64)},
		targetGenMap:     &rwMap{m: make(map[TargetKeyType]int64)},
	}
}

func (s *configReadinessCheckServer) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port)) // TODO: cathyzhyi
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	RegisterGenerationQueryServiceServer(srv, s)

	go func() {
		log.Printf("Server started on port %d", Port)
		if err := srv.Serve(listener); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
}

func (s *configReadinessCheckServer) GetCellTenantGeneration(ctx context.Context, req *GetGenerationReq) (*GetGenerationResp, error) {
	return getGeneration(s.cellTenantGenMap, req.Key)
}

func (s *configReadinessCheckServer) GetTargetGeneration(ctx context.Context, req *GetGenerationReq) (*GetGenerationResp, error) {
	return getGeneration(s.targetGenMap, req.Key)
}

func getGeneration(genMap *rwMap, key string) (*GetGenerationResp, error) {
	gen, ok := genMap.getKey(key)
	if !ok {
		gen = -1
	}
	return &GetGenerationResp{
		Generation: gen,
	}, nil
}

func (s *configReadinessCheckServer) UpdateCellTenantGeneration(b *config.CellTenant) {
	fmt.Printf("UpdateCellTenantGeneration %s: %d\n", b.Key().PersistenceString(), b.Generation)
	s.cellTenantGenMap.setKey(b.Key().PersistenceString(), b.Generation)
}

func (s *configReadinessCheckServer) UpdateTargetGeneration(t *config.Target) {
	fmt.Printf("UpdateCellTenantGeneration %s: %d\n", t.Key().PersistenceString(), t.Generation)
	s.cellTenantGenMap.setKey(t.Key().PersistenceString(), t.Generation) // TODO need to find a different key for target
}

func deleteNonExistObj(genMap *rwMap, ifExist func(key string) bool) {
	genMap.rangeKeys(func(key string) {
		if !ifExist(key) {
			fmt.Printf("delete %s\n", key)
			delete(genMap.m, key)
		}
	})
}

func (s *configReadinessCheckServer) CleanupStaleRecords(targets config.ReadonlyTargets) {
	s.DeleteNonExistCellTenants(func(key CellTenantKeyType) bool {
		cellTenantKey, err := config.CellTenantKeyFromPersistenceStringWithOutSlash(key)
		if err == nil {
			_, ok := targets.GetCellTenantByKey(cellTenantKey)
			return ok
		}
		return true // If the key format is wrong don't try to delete the entry
	})

	s.deleteNonExistTargets(func(key TargetKeyType) bool {
		targetKey, err := config.TargetKeyFromPersistenceStringWithOutSlash(key)
		if err == nil {
			_, ok := targets.GetTargetByKey(targetKey)
			return ok
		}
		return true // If the key format is wrong don't try to delete the entry
	})
}

func (s *configReadinessCheckServer) DeleteNonExistCellTenants(ifExist func(key CellTenantKeyType) bool) {
	deleteNonExistObj(s.cellTenantGenMap, ifExist)
}

func (s *configReadinessCheckServer) deleteNonExistTargets(ifExist func(key CellTenantKeyType) bool) {
	deleteNonExistObj(s.targetGenMap, ifExist)
}

func (m *rwMap) setKey(key string, value int64) {
	m.Lock()
	defer m.Unlock()
	m.m[key] = value
}

func (m *rwMap) getKey(key string) (int64, bool) {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.m[key]
	return v, ok
}

func (m *rwMap) rangeKeys(f func(key string)) {
	m.Lock()
	defer m.Unlock()

	for k := range m.m {
		f(k)
	}
}
