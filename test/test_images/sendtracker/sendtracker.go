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
	"net/http"
	"strconv"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
)

const doneSendTimeout = 5 * time.Minute

type rangeResult struct {
	startElapsed time.Duration
	success      bool
	statusCode   int
	minID        int
	maxID        int
}

type rangeResultArr []rangeResult

const (
	// Waiting for receiver to receive a connect message
	SendStateConnecting = iota
	// Waiting for the start request from the user
	SendStateReady
	// Sending events, waiting for stop
	SendStateSending
	// Sending stop message/waiting for receiver to finish
	SendStateFinishing
	// Hack.  Set by main to indicate that the receiver is done, so we can call Results
	// and ResultsReady will return true.
	SendStateDone
)

func (r rangeResultArr) String() string {
	result := fmt.Sprintf("rangeResults{\n")
	if len(r) == 0 {
		result += "  nil\n"
	}
	for i := range r {
		if r[i].success {
			result += "  success- "
		} else {
			result += "  failure- "
		}
		result += fmt.Sprintf("range: %d-%d statusCode: %d startElapsed %s\n", r[i].minID, r[i].maxID, r[i].statusCode, r[i].startElapsed)
	}
	result = result + "}\n"
	return result
}

// Responsibile for sending events
type sendPacer struct {
	client cloudevents.Client
	// Sleep between event sends
	delay time.Duration

	// Lock for updating state
	stateLock sync.Mutex
	// Current state
	state int
	// Indicates we have started
	start chan struct{}
	// Event ranges for successful and failed sends.
	// Single threaded, so not locked.
	results rangeResultArr
	// The receiver (hack used when seeing if our connect or Done message has made it through.
	receiver *Receiver
	// The summary to return through /result
	resultSummary string
	// The name of the source to use for the events.
	source string
}

func (s *sendPacer) getState() int {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	return s.state
}

// Return true if the state changed, false if it didn't change
// because the state is the same as the previous state, and error
// if it's an illegal transition.
func (s *sendPacer) setState(newstate int) (bool, error) {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	if s.state == newstate {
		return false, nil
	} else if s.state == newstate-1 {
		s.state = newstate
		return true, nil
	} else {
		return false, fmt.Errorf("illegal state transition %d -> %d", s.state, newstate)
	}
}

// Send connect messages to the receiver
func (s *sendPacer) runConnect() error {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("event-trace-connect")
	event.SetSource(s.source)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData("{}")
	idNum := 0

	connected := false
	startConnectTime := time.Now()
	for !connected {
		idNum++
		event.SetID(strconv.Itoa(idNum))
		rctx, _, err := s.client.Send(context.Background(), event)
		rtctx := cloudevents.HTTPTransportContextFrom(rctx)
		time.Sleep(time.Second)
		state := s.receiver.GetState()
		if state == ReceiveStateReady {
			connected = true
		}
		if !connected && time.Now().Sub(startConnectTime) > 2*time.Minute {
			return fmt.Errorf("connection startup failed (timeout), send err %v, code %v", err, rtctx.StatusCode)
		}
	}
	return nil
}

// Send the measured events to the receiver, recording the results for each.
func (s *sendPacer) runMeasure() error {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("event-trace-event")
	event.SetSource(s.source)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData("{}")
	idNum := 0

	startTime := time.Now()

	ending := false
	for !ending {
		idNum++

		if s.delay > 0 {
			time.Sleep(s.delay)
		}
		success := false
		event.SetID(strconv.Itoa(idNum))
		rctx, _, err := s.client.Send(context.Background(), event)
		rtctx := cloudevents.HTTPTransportContextFrom(rctx)
		if err == nil && rtctx.StatusCode >= http.StatusOK && rtctx.StatusCode < http.StatusMultipleChoices {
			success = true
		} else {
			success = false
		}
		if len(s.results) > 0 && s.results[len(s.results)-1].success == success && s.results[len(s.results)-1].statusCode == rtctx.StatusCode {
			s.results[len(s.results)-1].maxID = idNum
		} else {
			s.results = append(s.results, rangeResult{success: success, statusCode: rtctx.StatusCode, minID: idNum, maxID: idNum, startElapsed: time.Now().Sub(startTime)})
		}
		if s.getState() == SendStateFinishing {
			ending = true
		}
	}

	return nil
}

// Send done messages to the receiver.
func (s *sendPacer) runDone() error {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("event-trace-done")
	event.SetSource(s.source)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData("{}")
	idNum := 0

	done := false
	startDoneTime := time.Now()
	for !done {
		idNum++
		event.SetID(strconv.Itoa(idNum))
		rctx, _, err := s.client.Send(context.Background(), event)
		rtctx := cloudevents.HTTPTransportContextFrom(rctx)
		time.Sleep(time.Second)
		state := s.receiver.GetState()
		if state >= ReceiveStateFinishing {
			done = true
		}
		if !done && time.Now().Sub(startDoneTime) > 2*time.Minute {
			s.receiver.Done()
			return fmt.Errorf("Done send failed (timeout), send err %v, code %v", err, rtctx.StatusCode)
		}
	}
	return nil
}

// Run the entire send process
func (s *sendPacer) Run() error {
	fmt.Printf("Waiting to connect\n")
	err := s.runConnect()
	if err != nil {
		return err
	}
	_, err = s.setState(SendStateReady)
	if err != nil {
		return err
	}
	fmt.Printf("Waiting for start\n")
	<-s.start
	fmt.Printf("Started\n")

	err = s.runMeasure()
	if err != nil {
		return err
	}
	fmt.Printf("Sending done to receiver\n")
	err = s.runDone()
	if err != nil {
		return err
	}
	return nil
}

func (s *sendPacer) handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, err := s.setState(SendStateFinishing)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		res := fmt.Sprintf("invalid stop call: %v", err)
		fmt.Printf("%s\n", res)
		w.Write([]byte(res))
		return

	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))

}

func (s *sendPacer) handleStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	changed, err := s.setState(SendStateSending)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		res := fmt.Sprintf("invalid start call: %v", err)
		fmt.Printf("%s\n", res)
		w.Write([]byte(res))
		return
	}
	if changed {
		close(s.start)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))

}

func (s *sendPacer) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if s.getState() == SendStateReady {
		w.Write([]byte("true"))
	} else {
		w.Write([]byte("false"))
	}
}

func (s *sendPacer) handleResultsReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if s.getState() == SendStateDone {
		w.Write([]byte("true"))
	} else {
		w.Write([]byte("false"))
	}
}

func (s *sendPacer) handleResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	state := s.getState()
	if state != SendStateDone {
		w.WriteHeader(http.StatusBadRequest)
		res := fmt.Sprintf("bad state in get results %d", state)
		fmt.Printf("%s\n", res)
		w.Write([]byte(res))
		return
	} else {
		w.Write([]byte(s.resultSummary))
	}
}

func makeSender(client cloudevents.Client, receiver *Receiver, source string, delay time.Duration) *sendPacer {
	return &sendPacer{client: client,
		delay:    delay,
		start:    make(chan struct{}),
		receiver: receiver,
		source:   source,
	}
}
