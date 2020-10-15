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
	protoPayload = "protoPayload"
	serviceName  = "serviceName"
	methodName   = "methodName"
	resourceName = "resourceName"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &auditLogReceiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}

	fmt.Printf("ServiceName to match: %q.\n", r.ServiceName)
	fmt.Printf("MethodName to match: %q.\n", r.MethodName)
	fmt.Printf("ResourceName to match: %q.\n", r.ResourceName)
	fmt.Printf("Type to match: %q.\n", r.Type)
	fmt.Printf("Source to match: %q.\n", r.Source)
	fmt.Printf("Subject to match: %q.\n", r.Subject)

	return knockdown.Main(r.Config, r)
}

type auditLogReceiver struct {
	knockdown.Config

	ServiceName  string `envconfig:"SERVICENAME" required:"true"`
	MethodName   string `envconfig:"METHODNAME" required:"true"`
	ResourceName string `envconfig:"RESOURCENAME" required:"true"`
	Type         string `envconfig:"TYPE" required:"true"`
	Source       string `envconfig:"SOURCE" required:"true"`
	Subject      string `envconfig:"SUBJECT" required:"true"`
}

func (r *auditLogReceiver) Knockdown(event cloudevents.Event) bool {
	fmt.Printf("auditlogs target received event\n")
	fmt.Printf("event.Context is %s", event.Context.String())

	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data(), &eventData); err != nil {
		fmt.Printf("failed unmarshall event.Data %s.\n", err.Error())
	}
	payload := eventData[protoPayload].(map[string]interface{})
	eventDataServiceName := payload[serviceName].(string)
	fmt.Printf("event.Data.%s is %s \n", serviceName, eventDataServiceName)
	eventDataMethodName := payload[methodName].(string)
	fmt.Printf("event.Data.%s is %s \n", methodName, eventDataMethodName)
	eventDataResourceName := payload[resourceName].(string)
	fmt.Printf("event.Data.%s is %s \n", resourceName, eventDataResourceName)
	incorrectAttributes := make(map[string]lib.PropPair)

	if event.Type() != r.Type {
		incorrectAttributes[lib.EventType] = lib.PropPair{Expected: r.Type, Received: event.Type()}
	}
	if event.Source() != r.Source {
		incorrectAttributes[lib.EventSource] = lib.PropPair{Expected: r.Source, Received: event.Source()}
	}
	if event.Subject() != r.Subject {
		incorrectAttributes[lib.EventSubject] = lib.PropPair{Expected: r.Subject, Received: event.Subject()}
	}
	if eventDataServiceName != r.ServiceName {
		incorrectAttributes[serviceName] = lib.PropPair{Expected: r.ServiceName, Received: eventDataServiceName}
	}
	if eventDataMethodName != r.MethodName {
		incorrectAttributes[methodName] = lib.PropPair{Expected: r.MethodName, Received: eventDataMethodName}
	}
	if eventDataResourceName != r.ResourceName {
		incorrectAttributes[resourceName] = lib.PropPair{Expected: r.ResourceName, Received: eventDataResourceName}
	}

	if len(incorrectAttributes) == 0 {
		fmt.Printf("successfully sent to auditlogs target")
		return true
	}
	for k, v := range incorrectAttributes {
		fmt.Printf("%s doesn't match, event prop is %q while receiver prop is %q \n", k, v.Received, v.Expected)
	}
	return false
}
