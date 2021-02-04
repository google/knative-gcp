/*
Copyright 2021 Google LLC

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

package controller

import (
	"sync"

	. "github.com/google/knative-gcp/pkg/broker/readiness"
)

type record struct {
	genExpect int64
	podGens   map[podKeyType]int64 //pod to generation
	doneChan  chan interface{}
	genChan   chan int64
}

type recordsMap struct {
	sync.RWMutex
	records map[CellTenantKeyType]*record
}

func (m *recordsMap) getRecord(key CellTenantKeyType) (*record, bool) {
	m.RLock()
	defer m.RUnlock()
	r, ok := m.records[key]
	return r, ok
}

func (m *recordsMap) deleteRecord(key CellTenantKeyType) {
	m.RLock()
	defer m.RUnlock()
	delete(m.records, key)
}

func (m *recordsMap) addRecord(key CellTenantKeyType) *record {
	m.RLock()
	defer m.RUnlock()
	r := &record{
		podGens: make(map[podKeyType]int64),
	}
	m.records[key] = r
	return r
}
