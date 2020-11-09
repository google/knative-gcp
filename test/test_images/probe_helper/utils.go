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

package main

import (
	"fmt"
	"sync"
	"time"
)

// eventTimestamp is a synchronized wrapper around a timestamp.
type eventTimestamp struct {
	sync.RWMutex
	time time.Time
}

// setNow sets the desired timestamp to the current time.
func (t *eventTimestamp) setNow() {
	t.Lock()
	defer t.Unlock()
	t.time = time.Now()
}

// getTime gets the timestamp's time.
func (t *eventTimestamp) getTime() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.time
}

// receivedEventsMap is a sychronized map which holds waiting channels on each of
// the probe requests which are currently in the process of being handled.
type receivedEventsMap struct {
	sync.RWMutex
	channels map[string]chan bool
}

// createReceiverChannel creates a new channel at a given index in the receivedEventsMap.
// It is expected to fail if the index is already populated, i.e., if the probe
// request is duplicated.
func (r *receivedEventsMap) createReceiverChannel(channelID string) (chan bool, error) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.channels[channelID]; ok {
		return nil, fmt.Errorf("Receiver channel already exists for key %v", channelID)
	}
	receiverChannel := make(chan bool, 1)
	r.channels[channelID] = receiverChannel
	return receiverChannel, nil
}

// deleteReceiverChannel closes the waiting channel at a given index in the receivedEventsMap
// and deletes the map entry.
func (r *receivedEventsMap) deleteReceiverChannel(channelID string) {
	r.Lock()
	if ch, ok := r.channels[channelID]; ok {
		close(ch)
		delete(r.channels, channelID)
	}
	r.Unlock()
}
