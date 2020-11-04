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
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// Waiting for the sender to send a connect message
	ReceiveStateConnecting = iota
	// Receiving messages
	ReceiveStateReady
	// Receiving messages, but will stop after timeout
	ReceiveStateFinishing
	// Will not record any more messages
	ReceiveStateDone
)

type Receiver struct {
	startTime time.Time
	client    cloudevents.Client
	// Hold received message id's as we get them.  For
	// now only compress the results after taking data.
	// Revisit if memory is an issue.
	resultMap map[int]struct{}
	// Count of duplicates we see when adding to map
	resultDuplicates int
	// sumarized results once we are in Done
	resultList rangeReceivedArr
	// Lock for modifying resultMap
	resultLock sync.Mutex
	// Lock for modifying state
	stateLock sync.Mutex
	// state of the receiver
	state int
	// channel closed when we have reached the Done state.
	fullyDone chan struct{}
	// name of the source to accept events from
	receiveSource string
	// Time to wait between receiving the done message and stopping to listen
	// for events
	waitAfterDone time.Duration
}

func (r *Receiver) GetState() int {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	return r.state
}

// Transitions occur due to event deliveries that can be duplicated, so fail to change
// but don't error on invalid transitions.
func (r *Receiver) setState(newState int) bool {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	// Transition either in the normal order, or if the done call never succeeded and we are forced
	// to done.
	if r.state == newState-1 || newState == ReceiveStateDone && r.state == ReceiveStateReady {
		r.state = newState
		return true
	}
	return false
}

func (r *Receiver) GetResults() (rangeReceivedArr, int) {
	r.resultLock.Lock()
	defer r.resultLock.Unlock()
	state := r.GetState()
	if state != ReceiveStateDone {
		log.Fatalf("Bad state for get results: %d", state)
	}
	return r.resultList, r.resultDuplicates
}

func StartReceiver(receiveSource string, waitAfterDone time.Duration) *Receiver {
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}
	r := &Receiver{
		client:        client,
		startTime:     time.Now(),
		resultMap:     make(map[int]struct{}),
		fullyDone:     make(chan struct{}),
		receiveSource: receiveSource,
		waitAfterDone: waitAfterDone,
	}

	fmt.Printf("\nReceiver waiting to start\n")
	go func() {
		if err := r.client.StartReceiver(context.Background(), r.Receive); err != nil {
			log.Fatal(err)
		}
	}()
	return r
}

type rangeReceived struct {
	minID int
	maxID int
}

type rangeReceivedArr []rangeReceived

func (r rangeReceivedArr) String() string {
	result := fmt.Sprintf("rangeReceived{\n")
	if len(r) == 0 {
		result += "  nil\n"
	}
	for i := range r {
		result += fmt.Sprintf("  range: %d-%d\n", r[i].minID, r[i].maxID)
	}
	result = result + "}\n"
	return result
}

// Convert a map of ints into an ordered list of maximal length range
// extents for all elements in the map.
func mapToRange(mapIn map[int]struct{}) rangeReceivedArr {
	var listOut rangeReceivedArr
	allKey := make([]int, 0, len(mapIn))
	for id := range mapIn {
		allKey = append(allKey, id)
	}
	sort.Ints(allKey)

	for _, key := range allKey {
		if len(listOut) == 0 || listOut[len(listOut)-1].maxID+1 != key {
			listOut = append(listOut, rangeReceived{minID: key, maxID: key})
		} else {
			listOut[len(listOut)-1].maxID = key
		}
	}
	return listOut
}

// Called when we are fully done receiving events.  This either happens
// after a delay after we received the finished event, or is called directly
// by the sender if it never successfully gets a finish event to us.
func (r *Receiver) Done() {
	r.resultLock.Lock()
	changed := r.setState(ReceiveStateDone)

	// Only compute the list on the first Done
	if !changed {
		r.resultLock.Unlock()
		return
	}
	r.resultList = mapToRange(r.resultMap)
	r.resultLock.Unlock()
	close(r.fullyDone)
}

func (r *Receiver) Receive(ctx context.Context, event cloudevents.Event) {
	if event.Source() == r.receiveSource {
		switch event.Type() {
		case "event-trace-connect":
			r.setState(ReceiveStateReady)
			return
		case "event-trace-event":
			id, err := strconv.Atoi(event.ID())
			if err != nil {
				log.Fatalf("Bad ID Decoding: %v %s", err, event)
			}

			if id <= 0 {
				log.Fatalf("Invalid ID %d: %s", id, event)
			}

			r.resultLock.Lock()
			state := r.GetState()
			if state == ReceiveStateReady || state == ReceiveStateFinishing {
				_, exists := r.resultMap[id]
				if !exists {
					r.resultMap[id] = struct{}{}
				} else {
					r.resultDuplicates++
				}
			}
			r.resultLock.Unlock()

		case "event-trace-done":
			change := r.setState(ReceiveStateFinishing)
			// Only enqueue a wakeup to transition to Done once
			if change {
				fmt.Printf("Saw done request\n")
				// Wait to allow delayed event delivery to come in.
				time.AfterFunc(r.waitAfterDone, r.Done)
			}
			return
		default:
			log.Fatalf("Unexpected event type %s: %s", event.Type(), event)
			return
		}
	}
}
