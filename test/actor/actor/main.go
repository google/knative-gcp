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
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/net/netutil"
)

type config struct {
	// Delay is the delay before responding requests.
	Delay time.Duration `envconfig:"DELAY"`

	// DelayHosts is the list of hosts to delay (separated by ',').
	// If empty, then there will be no delay.
	// If set to "*", then all responses will be delayed.
	DelayHosts string `envconfig:"DELAY_HOSTS"`

	// ErrHosts is the list of hosts to return error 4xx (separated by ',).
	// If empty, then there will be no error.
	// If set to "*", then all responses will be error 4xx.
	ErrHosts string `envconfig:"ERR_HOSTS"`

	// ErrRate is the chance of getting an error response.
	// Must be between 0 and 100.
	ErrRate int `envconfig:"ERR_RATE"`

	// MaxConn is the max number of connections this code will handle at a time.
	MaxConn int `envconfig:"MAX_CONN"`

	// AggregatorAddr is the aggregator address.
	// If empty, then metrics won't be sent.
	AggregatorAddr string `envconfig:"AGGREGATOR_ADDR"`

	// ReportGap is a duration within which if there is no event received,
	// existing metrics will be reported to the aggregator.
	ReportGap time.Duration `envconfig:"REPORT_GAP" default:"1m"`
}

func main() {
	// Create a large heap allocation of 10 GiB.
	// In some cases, this helps to avoid noise from GC.
	// It doesn't really consume 10GiB memory. See explanation at:
	// https://blog.twitch.tv/en/2019/04/10/go-memory-ballast-how-i-learnt-to-stop-worrying-and-love-the-heap-26c2462549a2/
	_ = make([]byte, 10<<30)

	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env config: %v\n", err)
	}

	delayHosts := parseHosts(env.DelayHosts)
	errHosts := parseHosts(env.ErrHosts)

	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to create net listener: %v\n", err)
	}
	defer l.Close()
	if env.MaxConn > 0 {
		l = netutil.LimitListener(l, env.MaxConn)
	}

	reqCh := make(chan int, 5000)
	go func() {
		time.Sleep(time.Minute)
		var count int64
		reported := false
		for {
			select {
			case <-time.After(env.ReportGap):
				if reported {
					break
				}

				if env.AggregatorAddr == "" {
					log.Printf("Received %d events in total\n", atomic.LoadInt64(&count))
				} else {
					reportReq, err := http.NewRequest("POST", env.AggregatorAddr, nil)
					if err != nil {
						log.Printf("ERROR: failed to create report request: %v\n", err)
					}
					reportReq.Header.Set("role", "actor")
					reportReq.Header.Set("count", fmt.Sprintf("%d", atomic.LoadInt64(&count)))

					resp, err := http.DefaultClient.Do(reportReq)
					if err != nil {
						log.Printf("ERROR: failed to report metrics: %v\n", err)
					}
					if resp.StatusCode != http.StatusOK {
						log.Printf("ERROR: failed to report metrics: response code %d\n", resp.StatusCode)
					}
				}
				reported = true

			case <-reqCh:
				atomic.AddInt64(&count, 1)
			}
		}
	}()

	log.Fatal(http.Serve(l, &svr{
		delay:      env.Delay,
		delayHosts: delayHosts,
		errHosts:   errHosts,
		errRate:    env.ErrRate,
		reqCh:      reqCh,
	}))
}

type svr struct {
	delay      time.Duration
	delayHosts *matchHosts
	errHosts   *matchHosts
	errRate    int
	reqCh      chan int
}

func (s *svr) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("Received request with headers: %v\n", req.Header)
	s.reqCh <- 1

	if s.errHosts.include(req.Host) {
		if s.diceErr() {
			w.Header().Set("content-type", "text/plain")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(fmt.Sprintf("Injected error for host %q", req.Host)))
			return
		}
	}

	if s.delayHosts.include(req.Host) {
		time.Sleep(s.delay)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *svr) diceErr() bool {
	if s.errRate <= 0 {
		return false
	}

	if s.errRate >= 100 {
		return true
	}
	r := rand.Int31n(100)
	return int(r) < s.errRate
}

func parseHosts(hosts string) *matchHosts {
	if hosts == "*" {
		return &matchHosts{matchAll: true}
	}

	m := make(map[string]bool)
	hs := strings.Split(hosts, ",")
	for _, h := range hs {
		m[h] = true
	}

	return &matchHosts{allowList: m}
}

type matchHosts struct {
	matchAll  bool
	allowList map[string]bool
}

func (m *matchHosts) include(h string) bool {
	if m.matchAll {
		return true
	}
	_, ok := m.allowList[h]
	return ok
}
