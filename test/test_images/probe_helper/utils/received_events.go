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

package utils

import (
	"context"
	"fmt"
	"sync"
)

func NewSyncReceivedEvents() *SyncReceivedEvents {
	return &SyncReceivedEvents{
		Channels: map[string]chan bool{},
	}
}

// SyncReceivedEvents is a synchronized wrapped around a map of channels.
type SyncReceivedEvents struct {
	sync.RWMutex
	Channels map[string]chan bool
}

// CreateReceiverChannel creates a receiver channel at a given index in a map
// of receiver channels.
func (r *SyncReceivedEvents) CreateReceiverChannel(channelID string) (func(), error) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.Channels[channelID]; ok {
		return nil, fmt.Errorf("receiver channel already exists for key:" + channelID)
	}
	receiverChannel := make(chan bool, 1)
	r.Channels[channelID] = receiverChannel
	cleanupFunc := func() {
		r.Lock()
		defer r.Unlock()

		close(receiverChannel)
		delete(r.Channels, channelID)
	}
	return cleanupFunc, nil
}

// SignalReceiverChannel sends a closing signal to a receiver channel at a given
// index in a map of receiver channels.
func (r *SyncReceivedEvents) SignalReceiverChannel(channelID string) error {
	r.RLock()
	defer r.RUnlock()

	receiverChannel, ok := r.Channels[channelID]
	if !ok {
		return fmt.Errorf("failed to signal non-existent channel:" + channelID)
	}
	receiverChannel <- true
	return nil
}

// WaitOnReceiverChannel waits on a receiver channel at a given index until it
// receives something or until the context expires.
func (r *SyncReceivedEvents) WaitOnReceiverChannel(ctx context.Context, channelID string) error {
	r.RLock()
	receiverChannel, ok := r.Channels[channelID]
	r.RUnlock()
	if !ok {
		return fmt.Errorf("failed to wait on non-existent channel:" + channelID)
	}

	select {
	case <-receiverChannel:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for receiver channel")
	}
}
