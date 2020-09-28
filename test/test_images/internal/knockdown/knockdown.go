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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"golang.org/x/sync/errgroup"
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

type Receiver interface {
	Knockdown(event cloudevents.Event) bool
}

// Main should be called by the process' main function. It will run the knockdown test. The return
// value MUST be used in os.Exit().
func Main(config Config, kdr Receiver) int {
	p, err := http.New()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Waiting to receive event (timeout in %v)...\n", config.Time)
	eg, ctx := errgroup.WithContext(context.Background())
	pctx, pcancel := context.WithTimeout(ctx, config.Time)
	ctx, cancel := context.WithCancel(ctx)
	eg.Go(func() error {
		defer cancel()
		return p.OpenInbound(pctx)
	})
	eg.Go(func() error {
		var done bool
		for {
			msg, err := p.Receive(ctx)
			if err == io.EOF {
				if done {
					return nil
				}
				return errors.New("knockdown not done")
			}
			if err != nil {
				return err
			}
			e, err := binding.ToEvent(ctx, msg)
			if err != nil {
				msg.Finish(err)
				return err
			}
			if kdr.Knockdown(*e) {
				// Knockdown succeeded, mark done and
				// continue until reaching EOF. This
				// is necessary to allow the HTTP
				// receiver to process any queued
				// messages so that it can shutdown
				// gracefully.
				done = true
				pcancel()
			}
			msg.Finish(nil)
		}
	})
	err = eg.Wait()
	if err != nil {
		fmt.Printf("Error receiving event: %v\n", err)
		err = writeFailedTerminationMessage()
	} else {
		err = writeSuccessfulTerminationMessage()
	}
	if err != nil {
		fmt.Printf("Error writing termination message: %v\n", err)
		return 1
	}
	return 0
}

// writeSuccessfulTerminationMessage should be called to indicate this knock down process received
// all the expected events.
func writeSuccessfulTerminationMessage() error {
	return writeTerminationMessage(map[string]interface{}{
		"success": true,
	})
}

// writeFailedTerminationMessage should be called to indicate this knock down process did not
// receive all the expected events.
func writeFailedTerminationMessage() error {
	return writeTerminationMessage(map[string]interface{}{
		"success": false,
	})
}

// writeTerminationMessage writes the given result to the termination file, which is read by the e2e
// test framework to determine if this Pod received all the expected knockdown events.
func writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
