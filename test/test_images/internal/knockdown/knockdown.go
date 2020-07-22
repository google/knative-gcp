/*
Copyright 2019 Google LLC.

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

package knockdown

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Config is a common envconfig filled struct that includes the common options for knockdown
// testing. It is expected that knockdown tests will embed this struct in their configuration
// struct.
// E.g.
// type FooKnockdownConfig {
//     knockdown.Config
//     FooProperty string `envconfig:...`
// }
type Config struct {
	Time time.Duration `envconfig:"TIME" required:"true"`
}

// Main should be called by the process' main function. It will run the knockdown test. The return
// value MUST be used in os.Exit().
func Main(config Config, kdr Receiver) int {
	// Note that this only accepts TraceContext style tracing, not B3 style tracing. This is being
	// done here in test code so that we are effectively asserting that our components output in
	// TraceContext format. For all production code, kncloudevents.NewDefaultClient() should be used
	// instead.
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	r := receiver{
		kdr: kdr,
	}

	var ctx context.Context
	ctx, r.cancel = context.WithCancel(context.Background())

	timer := time.NewTimer(config.Time)
	defer timer.Stop()
	go func() {
		<-timer.C
		// Write the termination message if time out
		fmt.Printf("Time out waiting for the knock down event(s).")
		if err := r.writeFailedTerminationMessage(); err != nil {
			fmt.Printf("Failed to write termination message, %v\n", err)
		}
		r.cancel()
	}()

	fmt.Printf("Waiting to receive event (timeout in %s)...", config.Time.String())

	// Note, we assume that closing the context will gracefully shutdown the receiver. Having looked
	// at the current implementation, this is true. But nothing guarantees it in the future.
	if err := client.StartReceiver(ctx, r.Receive); err != nil {
		log.Fatal(err)
	}

	return 0
}

type receiver struct {
	kdr Receiver

	// cancel will cancel the main receiver's context. Calling this will cause Main() to become
	// unblocked and call os.Exit(0). In general, write{Successful,Failed}TerminationMessage should be
	// called first.
	cancel func()
}

type Receiver interface {
	Knockdown(event cloudevents.Event) bool
}

func (r *receiver) Receive(event cloudevents.Event) {
	done := r.kdr.Knockdown(event)
	if done {
		if err := r.writeSuccessfulTerminationMessage(); err != nil {
			fmt.Printf("Failed to write termination message, %s.\n", err.Error())
		}
		r.cancel()
		return
	}
	fmt.Printf("Event did not knockdown the process.")
}

// writeSuccessfulTerminationMessage should be called to indicate this knock down process received
// all the expected events.
func (r *receiver) writeSuccessfulTerminationMessage() error {
	return r.writeTerminationMessage(map[string]interface{}{
		"success": true,
	})
}

// writeFailedTerminationMessage should be called to indicate this knock down process did not
// receive all the expected events.
func (r *receiver) writeFailedTerminationMessage() error {
	return r.writeTerminationMessage(map[string]interface{}{
		"success": false,
	})
}

// writeTerminationMessage writes the given result to the termination file, which is read by the e2e
// test framework to determine if this Pod received all the expected knockdown events.
func (r *receiver) writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
