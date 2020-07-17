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
	context "context"
	"sync"
	"time"

	config "github.com/google/knative-gcp/pkg/broker/config"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/pkg/logging"
)

var BrokerConfigUnavailable error = brokerConfigUnavailable{}

type brokerConfigUnavailable struct{}

func (brokerConfigUnavailable) Error() string {
	return "broker config unavailable"
}

type BrokerWatch struct {
	brokerMut sync.RWMutex
	brokers   map[types.NamespacedName]*config.BrokerConfig
}

type BrokercellName types.NamespacedName

func NewBrokerWatch(ctx context.Context, brokercell BrokercellName, client BrokerControlClient) (*BrokerWatch, error) {
	t := &BrokerWatch{}
	go t.start(ctx, brokercell, client)
	return t, nil
}

func (t *BrokerWatch) start(ctx context.Context, brokercell BrokercellName, client BrokerControlClient) {
	var backoff *wait.Backoff
	for {
		if backoff == nil {
			backoff = &wait.Backoff{
				Duration: time.Duration(100 * time.Millisecond),
				Factor:   2,
				Jitter:   0.1,
				Steps:    7,
				Cap:      time.Duration(10 * time.Second),
			}
		} else {
			time.Sleep(backoff.Step())
		}
		watch, err := client.WatchBrokers(ctx, &WatchBrokersReq{BrokercellNamespace: brokercell.Namespace, BrokercellName: brokercell.Name})
		if err != nil {
			logging.FromContext(ctx).Error("failed to watch brokers", zap.Error(err), zap.Any("brokercell", brokercell))
			continue
		}

		resp, err := watch.Recv()
		if err != nil {
			logging.FromContext(ctx).Error("failed to receive initial broker state", zap.Error(err), zap.Any("brokercell", brokercell))
			continue
		}
		t.updateBrokers(resp)

		backoff = nil
		err = t.watchBrokers(watch)
		logging.FromContext(ctx).Error("error watching brokers", zap.Error(err), zap.Any("brokercell", brokercell))

		t.brokerMut.Lock()
		t.brokers = nil
		t.brokerMut.Unlock()
	}
}

func (t *BrokerWatch) watchBrokers(client BrokerControl_WatchBrokersClient) error {
	for {
		resp, err := client.Recv()
		if err != nil {
			return err
		}
		t.updateBrokers(resp)
	}
}

func (t *BrokerWatch) updateBrokers(r *WatchBrokersResp) {
	t.brokerMut.Lock()
	defer t.brokerMut.Unlock()
	if t.brokers == nil {
		t.brokers = make(map[types.NamespacedName]*config.BrokerConfig)
	}
	for _, u := range r.GetUpdates() {
		if u.Config == nil {
			delete(t.brokers, brokerNN(u.GetName()))
		} else {
			t.brokers[brokerNN(u.GetName())] = u.Config
		}
	}
}

func (t *BrokerWatch) GetBrokerConfig(broker types.NamespacedName) (*config.BrokerConfig, error) {
	t.brokerMut.RLock()
	defer t.brokerMut.RUnlock()
	if t.brokers == nil {
		return nil, BrokerConfigUnavailable
	}
	return t.brokers[broker], nil
}

func brokerNN(broker *config.BrokerName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: broker.Namespace,
		Name:      broker.Name,
	}
}
