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

package eventutil

import (
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"
)

func TestGetTTL(t *testing.T) {
	cases := []struct {
		name    string
		val     interface{}
		wantTTL int32
		wantOK  bool
	}{{
		name: "no ttl",
		val:  nil,
	}, {
		name: "invalid ttl",
		val:  "abc",
	}, {
		name:    "valid ttl",
		val:     100,
		wantOK:  true,
		wantTTL: 100,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			et := &TTL{Logger: zap.NewNop()}
			e := event.New()
			e.SetExtension(ttlAttribute, tc.val)
			gotTTL, gotOK := et.GetTTL(&e)
			if gotOK != tc.wantOK {
				t.Errorf("Found TTL OK got=%v, want=%v", gotOK, tc.wantOK)
			}
			if gotTTL != tc.wantTTL {
				t.Errorf("TTL got=%d, want=%d", gotTTL, tc.wantTTL)
			}
		})
	}
}

func TestDeleteTTL(t *testing.T) {
	et := &TTL{Logger: zap.NewNop()}
	e := event.New()
	e.SetExtension(ttlAttribute, 100)
	et.DeleteTTL(&e)
	_, ok := e.Extensions()[ttlAttribute]
	if ok {
		t.Error("After DeleteTTL TTL found got=true, want=false")
	}
}

func TestUpdateTTL(t *testing.T) {
	cases := []struct {
		name        string
		preemptive  int32
		existingTTL interface{}
		wantTTL     int32
	}{{
		name:        "no existing ttl",
		preemptive:  100,
		existingTTL: nil,
		wantTTL:     100,
	}, {
		name:        "invalid existing ttl",
		preemptive:  100,
		existingTTL: "abc",
		wantTTL:     100,
	}, {
		name:        "valid existing ttl",
		preemptive:  100,
		existingTTL: 10,
		wantTTL:     9,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			et := &TTL{Logger: zap.NewNop()}
			e := event.New()
			e.SetExtension(ttlAttribute, tc.existingTTL)
			et.UpdateTTL(&e, tc.preemptive)
			gotTTL, ok := et.GetTTL(&e)
			if !ok {
				t.Error("Found TTL after UpdateTTL got=false, want=true")
			}
			if gotTTL != tc.wantTTL {
				t.Errorf("TTL got=%d, want=%d", gotTTL, tc.wantTTL)
			}
		})
	}
}
