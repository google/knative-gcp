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
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
)

func TestGetRemainingHops(t *testing.T) {
	cases := []struct {
		name     string
		val      interface{}
		wantHops int32
		wantOK   bool
	}{{
		name: "no hops",
		val:  nil,
	}, {
		name: "invalid hops",
		val:  "abc",
	}, {
		name:     "valid hops",
		val:      100,
		wantOK:   true,
		wantHops: 100,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := event.New()
			e.SetExtension(HopsAttribute, tc.val)
			gotHops, gotOK := GetRemainingHops(context.Background(), &e)
			if gotOK != tc.wantOK {
				t.Errorf("Found Hops OK got=%v, want=%v", gotOK, tc.wantOK)
			}
			if gotHops != tc.wantHops {
				t.Errorf("Hops got=%d, want=%d", gotHops, tc.wantHops)
			}
		})
	}
}

func TestDeleteRemainingHops(t *testing.T) {
	e := event.New()
	e.SetExtension(HopsAttribute, 100)
	DeleteRemainingHops(context.Background(), &e)
	_, ok := e.Extensions()[HopsAttribute]
	if ok {
		t.Error("After DeleteRemainingHops Hops found got=true, want=false")
	}
}

func TestUpdateHops(t *testing.T) {
	cases := []struct {
		name         string
		preemptive   int32
		existingHops interface{}
		wantHops     int32
	}{{
		name:         "no existing hops",
		preemptive:   100,
		existingHops: nil,
		wantHops:     100,
	}, {
		name:         "invalid existing hops",
		preemptive:   100,
		existingHops: "abc",
		wantHops:     100,
	}, {
		name:         "valid existing hops",
		preemptive:   100,
		existingHops: 10,
		wantHops:     9,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := event.New()
			e.SetExtension(HopsAttribute, tc.existingHops)
			UpdateRemainingHops(context.Background(), &e, tc.preemptive)
			gotHops, ok := GetRemainingHops(context.Background(), &e)
			if !ok {
				t.Error("Found Hops after UpdateRemainingHops got=false, want=true")
			}
			if gotHops != tc.wantHops {
				t.Errorf("Hops got=%d, want=%d", gotHops, tc.wantHops)
			}
		})
	}
}
