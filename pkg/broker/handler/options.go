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

package handler

import (
	"runtime"
	"time"

	"cloud.google.com/go/pubsub"
)

var (
	defaultHandlerConcurrency     = runtime.NumCPU()
	defaultMaxConcurrencyPerEvent = 1
	defaultTimeout                = 10 * time.Minute

	// This is the pubsub default MaxExtension.
	// It would not make sense for handler timeout per event be greater
	// than this value because the message would be nacked before the handler
	// timeouts.
	// TODO: consider allow changing this value?
	maxTimeout = 10 * time.Minute
)

// Options holds all the options for create handler pool.
type Options struct {
	// HandlerConcurrency is the number of goroutines
	// will be spawned in each handler.
	HandlerConcurrency int
	// MaxConcurrencyPerEvent is the max number of goroutines
	// will be spawned to handle an event.
	MaxConcurrencyPerEvent int
	// TimeoutPerEvent is the timeout for handling an event.
	TimeoutPerEvent time.Duration
	// DeliveryTimeout is the timeout for delivering an event to a consumer.
	DeliveryTimeout time.Duration
	// PubsubReceiveSettings is the pubsub receive settings.
	PubsubReceiveSettings pubsub.ReceiveSettings
}

// NewOptions creates a Options.
func NewOptions(opts ...Option) (*Options, error) {
	opt := &Options{
		HandlerConcurrency:     defaultHandlerConcurrency,
		MaxConcurrencyPerEvent: defaultMaxConcurrencyPerEvent,
		TimeoutPerEvent:        defaultTimeout,
		PubsubReceiveSettings:  pubsub.DefaultReceiveSettings,
	}
	for _, o := range opts {
		o(opt)
	}
	return opt, nil
}

// Option is for providing individual option.
type Option func(*Options)

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
		if t > maxTimeout {
			o.TimeoutPerEvent = maxTimeout
		} else {
			o.TimeoutPerEvent = t
		}
	}
}

// WithPubsubReceiveSettings sets PubsubReceiveSettings.
func WithPubsubReceiveSettings(s pubsub.ReceiveSettings) Option {
	return func(o *Options) {
		o.PubsubReceiveSettings = s
	}
}

// WithDeliveryTimeout sets the DeliveryTimeout.
func WithDeliveryTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.DeliveryTimeout = t
	}
}
