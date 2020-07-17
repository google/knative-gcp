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
	"sync"

	"github.com/google/knative-gcp/pkg/broker/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"
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
	updateCh chan updates
}

type Server struct {
	UnimplementedBrokerControlServer

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
		updateCh: make(chan updates, 1),
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
	return w.watchUpdates(cell, watch)
}

func brokerName(broker types.NamespacedName) *config.BrokerName {
	return &config.BrokerName{
		Namespace: broker.Namespace,
		Name:      broker.Name,
	}
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
	}
}
