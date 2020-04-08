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

package context

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
)

func TestBrokerKey(t *testing.T) {
	_, err := GetBrokerKey(context.Background())
	if err != ErrBrokerKeyNotPresent {
		t.Errorf("error from GetBrokerKey got=%v, want=%v", err, ErrBrokerKeyNotPresent)
	}

	wantKey := "key"
	ctx := WithBrokerKey(context.Background(), wantKey)
	gotKey, err := GetBrokerKey(ctx)
	if err != nil {
		t.Errorf("unexpected error from GetBrokerKey: %v", err)
	}
	if gotKey != wantKey {
		t.Errorf("broker key from context got=%s, want=%s", gotKey, wantKey)
	}
}

func TestBroker(t *testing.T) {
	_, err := GetBroker(context.Background())
	if err != ErrBrokerNotPresent {
		t.Errorf("error from GetBroker got=%v, want=%v", err, ErrBrokerNotPresent)
	}

	wantBroker := &config.Broker{
		Name:      "broker",
		Namespace: "ns",
		Address:   "address",
		Targets: map[string]*config.Target{
			"target": &config.Target{
				Name:      "target",
				Namespace: "ns",
			},
		},
	}

	ctx := WithBroker(context.Background(), wantBroker)
	gotBroker, err := GetBroker(ctx)
	if err != nil {
		t.Errorf("unexpected error from GetBroker: %v", err)
	}

	if diff := cmp.Diff(wantBroker, gotBroker); diff != "" {
		t.Errorf("broker from context (-want,+got): %v", diff)
	}
}
