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

package main

import (
	"encoding/json"
	"fmt"

	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/test/lib"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
)

const (
	images    = "images"
	buildData = "Data"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &buildReceiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}

	fmt.Printf("Type to match: %q.\n", r.Type)
	fmt.Printf("Image to match: %q.\n", r.Images)

	return knockdown.Main(r.Config, r)
}

type buildReceiver struct {
	knockdown.Config

	Type   string `envconfig:"TYPE" required:"true"`
	Images string `envconfig:"IMAGES" required:"true"`
}

func (r *buildReceiver) Knockdown(event cloudevents.Event) bool {
	fmt.Printf("build target received event\n")
	fmt.Printf("event.Context is %s", event.Context.String())

	incorrectAttributes := make(map[string]lib.PropPair)

	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data(), &eventData); err != nil {
		fmt.Printf("failed unmarshall event.Data %s.\n", err.Error())
	}
	imageStr := fmt.Sprint(eventData[images])

	if event.Type() != r.Type {
		incorrectAttributes[lib.EventType] = lib.PropPair{Expected: r.Type, Received: event.Type()}
	}
	if imageStr != r.Images {
		incorrectAttributes[images] = lib.PropPair{Expected: r.Images, Received: imageStr}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		fmt.Printf("%s doesn't match, event prop is %q while receiver prop is %q \n", k, v.Received, v.Expected)
	}
	return false
}
