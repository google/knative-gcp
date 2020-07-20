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

package control

import (
	"errors"
	"math"
	"sync"

	"github.com/google/knative-gcp/pkg/broker/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
)

type brokerCell struct {
	brokersMut  sync.RWMutex
	brokers     map[types.NamespacedName]*config.BrokerConfig
	watchersMut sync.RWMutex
	watchers    []*watcher
}

type updates struct {
	brokers     []types.NamespacedName
	needsUpdate chan struct{}
}

type watcher struct {
	server         *Server
	updateCh       chan updates
	generationsMut sync.RWMutex
	generations    map[types.NamespacedName]int64
}

type Server struct {
	UnimplementedBrokerControlServer

	brokerQueueMut sync.RWMutex
	brokerQueue    workqueue.Interface

	brokerCellsMut sync.RWMutex
	brokerCells    map[types.NamespacedName]*brokerCell
}

func NewServer() *Server {
	return &Server{
		brokerCells: make(map[types.NamespacedName]*brokerCell),
	}
}

func (s *Server) WatchBrokers(req *WatchBrokersReq, watch BrokerControl_WatchBrokersServer) error {
	s.brokerCellsMut.RLock()
	cell, ok := s.brokerCells[types.NamespacedName{Namespace: req.BrokercellNamespace, Name: req.BrokercellName}]
	s.brokerCellsMut.RUnlock()
	if !ok {
		return status.Error(codes.NotFound, "brokercell not found")
	}

	w := watcher{
		server:      s,
		updateCh:    make(chan updates, 1),
		generations: make(map[types.NamespacedName]int64),
	}
	w.updateCh <- updates{needsUpdate: make(chan struct{})}

	cell.watchersMut.Lock()
	watcherNum := len(cell.watchers)
	cell.watchers = append(cell.watchers, &w)
	cell.watchersMut.Unlock()

	defer func() {
		cell.watchersMut.Lock()
		cell.watchers = append(cell.watchers[:watcherNum], cell.watchers[watcherNum+1:]...)
		cell.watchersMut.Unlock()
	}()

	cell.brokersMut.RLock()
	brokers := make([]*BrokerUpdate, 0, len(cell.brokers))
	for n, b := range cell.brokers {
		brokers = append(brokers, &BrokerUpdate{Name: brokerName(n), Config: b})
	}
	cell.brokersMut.RUnlock()
	if err := watch.Send(&WatchBrokersResp{Updates: brokers}); err != nil {
		return err
	}
	w.updateGenerations(brokers)
	return w.watchUpdates(cell, watch)
}

func brokerName(broker types.NamespacedName) *config.BrokerName {
	return &config.BrokerName{
		Namespace: broker.Namespace,
		Name:      broker.Name,
	}
}

func (s *Server) RegisterBrokerWorkqueue(queue workqueue.Interface) {
	s.brokerQueueMut.Lock()
	defer s.brokerQueueMut.Unlock()
	s.brokerQueue = queue
}

func (s *Server) UpsertBrokercell(brokercellName types.NamespacedName, brokers map[types.NamespacedName]*config.BrokerConfig) {
	s.brokerCellsMut.Lock()
	if cell, ok := s.brokerCells[brokercellName]; ok {
		s.brokerCellsMut.Unlock()
		cell.brokersMut.Lock()
		for n, b1 := range brokers {
			if b2 := cell.brokers[n]; b2 == nil || b2.Generation < b1.Generation {
				cell.brokers[n] = b1
				cell.sendUpdate(n)
			}
		}
		for n := range cell.brokers {
			if brokers[n] == nil {
				delete(cell.brokers, n)
				cell.sendUpdate(n)
			}
		}
		cell.brokersMut.Unlock()
	} else {
		s.brokerCells[brokercellName] = &brokerCell{brokers: brokers}
		s.brokerCellsMut.Unlock()
	}
}

func (s *Server) UpdateBrokerConfig(brokercellName types.NamespacedName, brokerName types.NamespacedName, brokerConfig *config.BrokerConfig) error {
	s.brokerCellsMut.Lock()
	if cell, ok := s.brokerCells[brokercellName]; ok {
		s.brokerCellsMut.Unlock()
		cell.brokersMut.Lock()
		if brokerConfig == nil {
			if _, ok = cell.brokers[brokercellName]; ok {
				delete(cell.brokers, brokercellName)
				cell.sendUpdate(brokercellName)
			}
		} else if oldConfig := cell.brokers[brokercellName]; oldConfig == nil || brokerConfig.Generation > oldConfig.Generation {
			cell.brokers[brokercellName] = brokerConfig
			cell.sendUpdate(brokercellName)
		}
		cell.brokersMut.Unlock()
	} else {
		s.brokerCellsMut.Unlock()
		return errors.New("brokercell config not initialized")
	}
	return nil
}

func (s *Server) MinGeneration(brokercell types.NamespacedName, broker types.NamespacedName) int64 {
	var minGeneration int64 = math.MaxInt64
	s.brokerCellsMut.RLock()
	cell := s.brokerCells[brokercell]
	if cell == nil {
		return -1
	}
	s.brokerCellsMut.RUnlock()
	cell.watchersMut.RLock()
	watchers := cell.watchers
	cell.watchersMut.RUnlock()
	for _, watcher := range watchers {
		watcher.generationsMut.RLock()
		if g, ok := watcher.generations[broker]; ok && g < minGeneration {
			minGeneration = g
		} else {
			minGeneration = -1
		}
		watcher.generationsMut.RUnlock()
	}
	return minGeneration
}

func (s *Server) enqueueBroker(brokerName types.NamespacedName) {
	s.brokerQueueMut.RLock()
	queue := s.brokerQueue
	s.brokerQueueMut.RUnlock()
	if queue != nil {
		queue.Add(brokerName)
	}
}

func (b *brokerCell) sendUpdate(broker types.NamespacedName) {
	b.watchersMut.RLock()
	watchers := b.watchers
	b.watchersMut.RUnlock()
	for _, w := range watchers {
		u := <-w.updateCh
		if u.brokers == nil {
			close(u.needsUpdate)
		}
		u.brokers = append(u.brokers, broker)
		w.updateCh <- u
	}
}

func (w *watcher) updateGenerations(updates []*BrokerUpdate) {
	w.generationsMut.Lock()
	defer w.generationsMut.Unlock()
	for _, u := range updates {
		n := brokerNN(u.Name)
		if u.Config == nil {
			delete(w.generations, n)
		} else {
			w.generations[n] = u.Config.GetGeneration()
		}
		w.server.enqueueBroker(n)
	}
}

func (w *watcher) watchUpdates(cell *brokerCell, watch BrokerControl_WatchBrokersServer) error {
	for {
		u := <-w.updateCh
		if u.brokers == nil {
			w.updateCh <- u
			<-u.needsUpdate
			continue
		}
		w.updateCh <- updates{needsUpdate: make(chan struct{})}
		resp := WatchBrokersResp{
			Updates: make([]*BrokerUpdate, 0, len(u.brokers)),
		}
		cell.brokersMut.RLock()
		for _, n := range u.brokers {
			resp.Updates = append(resp.Updates, &BrokerUpdate{Name: brokerName(n), Config: cell.brokers[n]})
		}
		cell.brokersMut.RUnlock()
		if err := watch.Send(&resp); err != nil {
			return err
		}
		w.updateGenerations(resp.Updates)
	}
}
