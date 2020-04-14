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
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"
)

func TestWithProjectID(t *testing.T) {
	want := "random"
	opt := NewOptions(WithProjectID(want))
	if opt.ProjectID != want {
		t.Errorf("options project id got=%s, want=%s", opt.ProjectID, want)
	}
}

func TestWithHandlerConcurrency(t *testing.T) {
	want := 10
	opt := NewOptions(WithHandlerConcurrency(want))
	if opt.HandlerConcurrency != want {
		t.Errorf("options handler concurrency got=%d, want=%d", opt.HandlerConcurrency, want)
	}
}

func TestWithMaxConcurrency(t *testing.T) {
	want := 10
	opt := NewOptions(WithMaxConcurrentPerEvent(want))
	if opt.MaxConcurrencyPerEvent != want {
		t.Errorf("options max concurrency per event got=%d, want=%d", opt.MaxConcurrencyPerEvent, want)
	}
}

func TestWithTimeout(t *testing.T) {
	want := 10 * time.Minute
	opt := NewOptions(WithTimeoutPerEvent(want))
	if opt.TimeoutPerEvent != want {
		t.Errorf("options timeout per event got=%v, want=%v", opt.TimeoutPerEvent, want)
	}
}

func TestWithReceiveSettings(t *testing.T) {
	want := pubsub.ReceiveSettings{
		NumGoroutines: 10,
		MaxExtension:  time.Minute,
	}
	opt := NewOptions(WithPubsubReceiveSettings(want))
	if diff := cmp.Diff(want, opt.PubsubReceiveSettings); diff != "" {
		t.Errorf("options ReceiveSettings (-want,+got): %v", diff)
	}
}

func TestWithPubsubClient(t *testing.T) {
	opt := NewOptions()
	if opt.PubsubClient != nil {
		t.Errorf("options PubsubClient got=%v, want=nil", opt.PubsubClient)
	}
	opt = NewOptions(WithPubsubClient(&pubsub.Client{}))
	if opt.PubsubClient == nil {
		t.Error("options PubsubClient got=nil, want=non-nil client")
	}
}

func TestWithSignal(t *testing.T) {
	opt := NewOptions(WithSyncSignal(make(chan struct{})))
	if opt.SyncSignal == nil {
		t.Error("options SyncSignal is nil")
	}
}
