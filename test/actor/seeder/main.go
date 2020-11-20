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
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
)

type config struct {
	// Target is the address to seed events.
	Target string `envconfig:"TARGET"`

	// Interval is the interval between seeding batch of events.
	Interval time.Duration `envconfig:"INTERVAL"`

	// Concurrency is the concurrency when seeding a batch of events.
	Concurrency int `envconfig:"CONCURRENCY" default:"1"`

	// Extensions to add to the event.
	// In the format of "key1:value1;key2:values".
	Extensions string `envconfig:"EXTENSIONS"`

	// Size is the payload size of events.
	Size int64 `envconfig:"SIZE" default:"100"`

	// AggregatorAddr is the aggregator address.
	// If empty, then metrics won't be sent.
	AggregatorAddr string `envconfig:"AGGREGATOR_ADDR"`

	// Elapse is total time of seeding events.
	Elapse time.Duration `envconfig:"ELAPSE" default:"9999h"`
}

func main() {
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env config: %v\n", err)
	}

	// Hack the connection reuse.
	// This helps prevent DNS errors to some extent.
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 80

	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("Failed to create cloudevents client: %v\n", err)
	}

	ext := map[string]string{}
	kvs := strings.Split(env.Extensions, ";")
	for _, kv := range kvs {
		p := strings.Split(kv, ":")
		if len(p) == 2 {
			ext[p[0]] = p[1]
		}
	}

	data := make([]byte, env.Size)
	rand.Read(data)

	endTime := time.Now().Add(env.Elapse)
	var success, failure int64
	successCh := make(chan int, env.Concurrency)
	failureCh := make(chan int, env.Concurrency)

	go func() {
		for {
			select {
			case <-successCh:
				atomic.AddInt64(&success, 1)
			case <-failureCh:
				atomic.AddInt64(&failure, 1)
			}
		}
	}()

	for time.Now().Before(endTime) {
		e := event.New()
		e.SetID(uuid.New().String())
		e.SetSource("ce-test-actor.seeder")
		e.SetType("seed")
		e.SetSubject("tick")
		e.SetTime(time.Now())
		if env.Size > 0 {
			e.SetData("application/octet-stream", data)
		}

		for k, v := range ext {
			e.SetExtension(k, v)
		}

		for i := 0; i < env.Concurrency; i++ {
			go func() {
				_, ret := client.Request(cecontext.WithTarget(context.Background(), env.Target), e)
				if protocol.IsACK(ret) {
					successCh <- 1
					log.Printf("Successfully seeded event (id=%s) to target %q\n", e.ID(), env.Target)
				} else {
					failureCh <- 1
					log.Printf("ERROR: Failed to seed event (id=%s) to target %q: %v\n", e.ID(), env.Target, ret.Error())
				}
			}()
		}

		log.Println("Sleeping...")
		time.Sleep(env.Interval)
	}

	// To finish all counting.
	time.Sleep(30 * time.Second)

	if env.AggregatorAddr == "" {
		log.Printf("Successfully sent events: %d\n", atomic.LoadInt64(&success))
		log.Printf("Failed to send events: %d\n", atomic.LoadInt64(&failure))
	} else {
		reportReq, err := http.NewRequest("POST", env.AggregatorAddr, nil)
		if err != nil {
			log.Printf("ERROR: failed to create report request: %v\n", err)
		}
		reportReq.Header.Set("role", "seeder")
		reportReq.Header.Set("success", fmt.Sprintf("%d", atomic.LoadInt64(&success)))
		reportReq.Header.Set("failure", fmt.Sprintf("%d", atomic.LoadInt64(&failure)))

		resp, err := http.DefaultClient.Do(reportReq)
		if err != nil {
			log.Printf("ERROR: failed to report metrics: %v\n", err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("ERROR: failed to report metrics: response code %d\n", resp.StatusCode)
		}
	}

	time.Sleep(10000 * time.Hour)
}
