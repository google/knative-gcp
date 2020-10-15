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
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/test/lib"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &pubsubReceiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}
	fmt.Printf("Type to match: %q.\n", r.Type)
	fmt.Printf("Source to match: %q.\n", r.Source)
	return knockdown.Main(r.Config, r)
}

type pubsubReceiver struct {
	knockdown.Config
	Type   string `envconfig:"TYPE" required:"true"`
	Source string `envconfig:"SOURCE" required:"true"`
	Schema string `envconfig:"SCHEMA" required:"false"`
}

func (r *pubsubReceiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	fmt.Printf("pubsub target received event\n")
	fmt.Printf("context of event is: %v\n", event.Context.String())

	incorrectAttributes := make(map[string]lib.PropPair)

	// Check type
	if event.Type() != r.Type {
		incorrectAttributes[r.Type] = lib.PropPair{Expected: r.Type, Received: event.Type()}
	}

	// Check source
	if event.Source() != r.Source {
		incorrectAttributes[lib.EventSource] = lib.PropPair{Expected: r.Source, Received: event.Source()}
	}

	// Check schema
	if event.DataSchema() != r.Schema {
		incorrectAttributes[lib.EventDataSchema] = lib.PropPair{Expected: r.Schema, Received: event.DataSchema()}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		fmt.Println(k, "expected:", v.Expected, "got:", v.Received)
	}
	return false
}
