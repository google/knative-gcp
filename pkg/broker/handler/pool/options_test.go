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
	opt, err := NewOptions(WithProjectID(want))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.ProjectID != want {
		t.Errorf("options project id got=%s, want=%s", opt.ProjectID, want)
	}
}

func TestWithHandlerConcurrency(t *testing.T) {
	want := 10
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err := NewOptions(WithHandlerConcurrency(want), WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.HandlerConcurrency != want {
		t.Errorf("options handler concurrency got=%d, want=%d", opt.HandlerConcurrency, want)
	}
}

func TestWithMaxConcurrency(t *testing.T) {
	want := 10
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err := NewOptions(WithMaxConcurrentPerEvent(want), WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.MaxConcurrencyPerEvent != want {
		t.Errorf("options max concurrency per event got=%d, want=%d", opt.MaxConcurrencyPerEvent, want)
	}
}

func TestWithTimeout(t *testing.T) {
	want := 10 * time.Minute
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err := NewOptions(WithTimeoutPerEvent(want), WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.TimeoutPerEvent != want {
		t.Errorf("options timeout per event got=%v, want=%v", opt.TimeoutPerEvent, want)
	}
}

func TestWithReceiveSettings(t *testing.T) {
	want := pubsub.ReceiveSettings{
		NumGoroutines: 10,
		MaxExtension:  time.Minute,
	}
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err := NewOptions(WithPubsubReceiveSettings(want), WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, opt.PubsubReceiveSettings); diff != "" {
		t.Errorf("options ReceiveSettings (-want,+got): %v", diff)
	}
}

func TestWithPubsubClient(t *testing.T) {
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err := NewOptions(WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.PubsubClient != nil {
		t.Errorf("options PubsubClient got=%v, want=nil", opt.PubsubClient)
	}
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err = NewOptions(WithPubsubClient(&pubsub.Client{}), WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.PubsubClient == nil {
		t.Error("options PubsubClient got=nil, want=non-nil client")
	}
}

func TestWithSignal(t *testing.T) {
	// Always add project id because the default value can only be retrieved on GCE/GKE machines.
	opt, err := NewOptions(WithSyncSignal(make(chan struct{})), WithProjectID("pid"))
	if err != nil {
		t.Errorf("NewOptions got unexpected error: %v", err)
	}
	if opt.SyncSignal == nil {
		t.Error("options SyncSignal is nil")
	}
}
