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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"github.com/kelseyhightower/envconfig"
)

const (
	eventType    = "type"
	eventSource  = "source"
	eventSubject = "subject"
	protoPayload = "protoPayload"
	serviceName  = "serviceName"
	methodName   = "methodName"
	resourceName = "resourceName"
)

func main() {
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	r := Receiver{}
	if err := envconfig.Process("", &r); err != nil {
		panic(err)
	}

	fmt.Printf("ServiceName to match: %q.\n", r.ServiceName)
	fmt.Printf("MethodName to match: %q.\n", r.MethodName)
	fmt.Printf("ResourceName to match: %q.\n", r.ResourceName)
	fmt.Printf("Type to match: %q.\n", r.Type)
	fmt.Printf("Source to match: %q.\n", r.Source)
	fmt.Printf("Subject to match: %q.\n", r.Subject)

	// Create a timer
	duration, _ := strconv.Atoi(r.Time)
	timer := time.NewTimer(time.Second * time.Duration(duration))
	defer timer.Stop()
	go func() {
		<-timer.C
		// Write the termination message if time out
		fmt.Printf("time out to wait for event with type %q source %q subject %q service_name %q method_name %q resource_name %q .\n",
			r.Type, r.Source, r.Subject, r.ServiceName, r.MethodName, r.ResourceName)
		if err := r.writeTerminationMessage(map[string]interface{}{
			"success": false,
		}); err != nil {
			fmt.Printf("failed to write termination message, %s.\n", err.Error())
		}
		os.Exit(0)
	}()

	if err := client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

type Receiver struct {
	ServiceName  string `envconfig:"SERVICENAME" required:"true"`
	MethodName   string `envconfig:"METHODNAME" required:"true"`
	ResourceName string `envconfig:"RESOURCENAME" required:"true"`
	Type         string `envconfig:"TYPE" required:"true"`
	Source       string `envconfig:"SOURCE" required:"true"`
	Subject      string `envconfig:"SUBJECT" required:"true"`
	Time         string `envconfig:"TIME" required:"true"`
}

type propPair struct {
	eventProp    string
	receiverProp string
}

func (r *Receiver) Receive(event cloudevents.Event) {
	fmt.Printf("event.Context is %s", event.Context.String())
	var eventData map[string]interface{}
	if err := json.Unmarshal(event.Data.([]byte), &eventData); err != nil {
		fmt.Printf("failed unmarshall event.Data %s.\n", err.Error())
	}
	payload := eventData[protoPayload].(map[string]interface{})
	eventDataServiceName := payload[serviceName].(string)
	fmt.Printf("event.Data.%s is %s \n", serviceName, eventDataServiceName)
	eventDataMethodName := payload[methodName].(string)
	fmt.Printf("event.Data.%s is %s \n", methodName, eventDataMethodName)
	eventDataResourceName := payload[resourceName].(string)
	fmt.Printf("event.Data.%s is %s \n", resourceName, eventDataResourceName)
	unmatchedProps := make(map[string]propPair)

	if event.Context.GetType() != r.Type {
		unmatchedProps[eventType] = propPair{event.Context.GetType(), r.Type}
	}
	if event.Context.GetSource() != r.Source {
		unmatchedProps[eventSource] = propPair{event.Context.GetSource(), r.Source}
	}
	if event.Context.GetSubject() != r.Subject {
		unmatchedProps[eventSubject] = propPair{event.Context.GetSubject(), r.Subject}
	}
	if eventDataServiceName != r.ServiceName {
		unmatchedProps[serviceName] = propPair{eventDataServiceName, r.ServiceName}
	}
	if eventDataMethodName != r.MethodName {
		unmatchedProps[methodName] = propPair{eventDataMethodName, r.MethodName}
	}
	if eventDataResourceName != r.ResourceName {
		unmatchedProps[resourceName] = propPair{eventDataResourceName, r.ResourceName}
	}

	if len(unmatchedProps) == 0 {
		// Write the termination message if the subject successfully matches
		if err := r.writeTerminationMessage(map[string]interface{}{
			"success": true,
		}); err != nil {
			fmt.Printf("failed to write termination message, %s.\n", err.Error())
		}
		os.Exit(0)
	} else {
		for k, v := range unmatchedProps {
			fmt.Printf("%s doesn't match, event prop is %q while receiver prop is %q \n", k, v.eventProp, v.receiverProp)
		}
	}
}

func (r *Receiver) writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
