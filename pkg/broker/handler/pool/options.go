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

package pool

import (
	"runtime"
	"time"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
)

var (
	defaultHandlerConcurrency     = runtime.NumCPU()
	defaultMaxConcurrencyPerEvent = 1
	defaultTimeout                = 10 * time.Minute
)

var defaultCeClient ceclient.Client

func init() {
	p, err := cev2.NewHTTP()
	if err != nil {
		panic(err)
	}
	c, err := cev2.NewClientObserved(p,
		ceclient.WithUUIDs(),
		ceclient.WithTimeNow(),
		ceclient.WithTracePropagation(),
	)
	if err != nil {
		panic(err)
	}
	defaultCeClient = c
}

// Options holds all the options for create handler pool.
type Options struct {
	// ProjectID is the project for pubsub.
	ProjectID string
	// HandlerConcurrency is the number of goroutines
	// will be spawned in each handler.
	HandlerConcurrency int
	// MaxConcurrencyPerEvent is the max number of goroutines
	// will be spawned to handle an event.
	MaxConcurrencyPerEvent int
	// TimeoutPerEvent is the timeout for handling an event.
	TimeoutPerEvent time.Duration
	// PubsubReceiveSettings is the pubsub receive settings.
	PubsubReceiveSettings pubsub.ReceiveSettings
	// EventRequester is the cloudevents client to deliver events.
	EventRequester ceclient.Client
	// SyncSignal is the signal to sync handler pool.
	SyncSignal <-chan struct{}
}

// NewOptions creates a Options.
func NewOptions(opts ...Option) *Options {
	opt := &Options{
		HandlerConcurrency:     defaultHandlerConcurrency,
		MaxConcurrencyPerEvent: defaultMaxConcurrencyPerEvent,
		TimeoutPerEvent:        defaultTimeout,
		PubsubReceiveSettings:  pubsub.DefaultReceiveSettings,
	}
	for _, o := range opts {
		o(opt)
	}
	if opt.EventRequester == nil {
		opt.EventRequester = defaultCeClient
	}
	return opt
}

// Option is for providing individual option.
type Option func(*Options)

// WithProjectID sets project ID.
func WithProjectID(id string) Option {
	return func(o *Options) {
		o.ProjectID = id
	}
}

// WithHandlerConcurrency sets HandlerConcurrency.
func WithHandlerConcurrency(c int) Option {
	return func(o *Options) {
		o.HandlerConcurrency = c
	}
}

// WithMaxConcurrentPerEvent sets MaxConcurrencyPerEvent.
func WithMaxConcurrentPerEvent(c int) Option {
	return func(o *Options) {
		o.MaxConcurrencyPerEvent = c
	}
}

// WithTimeoutPerEvent sets TimeoutPerEvent.
func WithTimeoutPerEvent(t time.Duration) Option {
	return func(o *Options) {
		o.TimeoutPerEvent = t
	}
}

// WithPubsubReceiveSettings sets PubsubReceiveSettings.
func WithPubsubReceiveSettings(s pubsub.ReceiveSettings) Option {
	return func(o *Options) {
		o.PubsubReceiveSettings = s
	}
}

// WithSyncSignal sets SyncSignal.
func WithSyncSignal(s <-chan struct{}) Option {
	return func(o *Options) {
		o.SyncSignal = s
	}
}
