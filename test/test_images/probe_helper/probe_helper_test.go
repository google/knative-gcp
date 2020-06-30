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
	"os"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// the port that the dummy broker listens to
	dummyBrokerPort = 9999
	// the port that the probe helper listens to
	probeHelperPort = 8070
	// the port that the probe receiver component listens to
	probeReceiverPort = 8080
)

// A helper function that starts a dummy broker which receives events forwarded by the probe helper and delivers the events
// back to the probe helper's receive port
func runDummyBroker(t * testing.T) {
	probeReceiverURL := fmt.Sprintf("http://localhost:%d", probeReceiverPort)
	bp, err := cloudevents.NewHTTP(cloudevents.WithPort(dummyBrokerPort), cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		t.Fatalf("Failed to create http protocol of the dummy broker, %v", err)
	}
	bc, err  := cloudevents.NewClient(bp)
	if err != nil {
		t.Fatal("Failed to create the dummy broker client, ", err)
	}
	bc.StartReceiver(context.Background(), func(event cloudevents.Event) {
		if res := bc.Send(context.Background(), event); !cloudevents.IsACK(res) {
			t.Fatalf("Failed to send cloudevent from the dummy broker: %v", res)
		}
	})
}

// The Unit test that test whether a cloud event sending to the probe helper can forward the event to the dummy broker
// and receive the event from the dummy broker
func TestDeliverEventsProbeHelper(t *testing.T) {
	// setup a dummy broker URL to be the receive component directly
	os.Setenv("K_SINK", fmt.Sprintf("http://localhost:%d", dummyBrokerPort))
	// start the dummy probe helper
	runProbeHelper()
	// start the dummy broker in a new goroutine
	go	runDummyBroker(t)
	probeHelperURL := fmt.Sprintf("http://localhost:%d", probeHelperPort)
	p, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeHelperURL))
	if err != nil {
		t.Fatalf("failed to create http protocol of the testing client : %s", err.Error())
	}
	c, err := cloudevents.NewClient(p)
	if err != nil {
		t.Fatalf("failed to create testing client: %s", err.Error())
	}
	// create a dummy cloud event and send the event to the dummy broker
	event := cloudevents.NewEvent()
	event.SetID("dummyID")
	event.SetType("dummyType")
	event.SetSource("dummySource")
	if result := c.Send(context.Background(), event); !cloudevents.IsACK(result) {
		t.Fatalf("failed to send cloudevent: %s", result.Error())
	}
}
