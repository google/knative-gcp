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
	"fmt"
	"sync"
)

// ReceivedEventsMap is a sychronized map which holds waiting channels on each of
// the probe requests which are currently in the process of being handled.
type ReceivedEventsMap struct {
	sync.RWMutex
	Channels map[string]chan bool
}

// CreateReceiverChannel creates a new channel at a given index in the ReceivedEventsMap.
// It is expected to fail if the index is already populated, i.e., if the probe
// request is duplicated.
func (r *ReceivedEventsMap) CreateReceiverChannel(channelID string) (chan bool, error) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.Channels[channelID]; ok {
		return nil, fmt.Errorf("Receiver channel already exists for key %v", channelID)
	}
	receiverChannel := make(chan bool, 1)
	r.Channels[channelID] = receiverChannel
	return receiverChannel, nil
}

// DeleteReceiverChannel closes the waiting channel at a given index in the receivedEventsMap
// and deletes the map entry.
func (r *ReceivedEventsMap) DeleteReceiverChannel(channelID string) {
	r.Lock()
	if ch, ok := r.Channels[channelID]; ok {
		close(ch)
		delete(r.Channels, channelID)
	}
	r.Unlock()
}
