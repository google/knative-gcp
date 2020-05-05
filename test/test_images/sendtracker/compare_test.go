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
	"testing"
	"time"
)

type testCompareInfo struct {
	name        string
	sendInfo    rangeResultArr
	receiveInfo rangeReceivedArr
	// This will result in errorString out
	errIn error

	// These indicate expected error
	missingElement bool
	missingRange   bool
	emptySend      bool

	// These indicate expected warning
	extraElement bool
	extraRange   bool
}

var compTests = []testCompareInfo{
	{
		name: "Match",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 100},
			{120, 190},
		},
	},
	{
		name: "ErrorIn",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 100},
			{120, 190},
		},
		errIn: fmt.Errorf("Inbound error"),
	},
	{
		name:        "EmptySendReceive",
		sendInfo:    rangeResultArr{},
		receiveInfo: rangeReceivedArr{},
		emptySend:   true,
	},
	{
		name:        "EmptySendNonemptyReceive",
		sendInfo:    rangeResultArr{},
		receiveInfo: rangeReceivedArr{{1, 10}},
		emptySend:   true,
		extraRange:  true,
	},
	{
		name: "MissingRange1",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
		},
		receiveInfo:  rangeReceivedArr{},
		missingRange: true,
	},
	{
		name: "MissingRange2",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 100},
		},
		missingRange: true,
	},
	{
		name: "MissingRange3",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{120, 190},
		},
		missingRange: true,
	},
	{
		name: "MissingElement1",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 99},
			{120, 190},
		},
		missingElement: true,
	},
	{
		name: "MissingElement2",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{2, 100},
			{120, 190},
		},
		missingElement: true,
	},
	{
		name: "ExtraElement1",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 101},
			{120, 190},
		},
		extraElement: true,
	},
	{
		name: "ExtraElement2",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 100},
			{119, 190},
		},
		extraElement: true,
	},
	{
		name: "ExtraRange",
		sendInfo: rangeResultArr{
			{time.Second, true, 202, 1, 100},
			{time.Second, false, 500, 100, 119},
			{time.Second, true, 202, 120, 190},
		},
		receiveInfo: rangeReceivedArr{
			{1, 100},
			{109, 110},
			{120, 190},
		},
		extraRange: true,
	},
}

// Test that the sender/receiver range comparison function works
func TestCompare(t *testing.T) {
	for _, test := range compTests {
		t.Run(test.name, func(t *testing.T) {
			errorString, warningString := compareLogs(test.sendInfo, test.receiveInfo, test.errIn)
			if test.errIn != nil || test.missingElement || test.missingRange || test.emptySend {
				if len(errorString) == 0 {
					t.Fatal("Missing expected error")
				}
			} else {
				if len(errorString) != 0 {
					t.Fatalf("Saw unexpected error: (%s)", errorString)
				}
			}

			if test.extraElement || test.extraRange {
				if len(warningString) == 0 {
					t.Fatal("Missing expected warning")
				}
			} else {
				if len(warningString) != 0 {
					t.Fatalf("Saw unexpected warning: (%s)", warningString)
				}
			}
		})
	}
}

type testMapToRange struct {
	name     string
	mapIn    map[int]struct{}
	expected rangeReceivedArr
}

var mapRangeTests = []testMapToRange{
	{
		name:     "empty",
		mapIn:    map[int]struct{}{},
		expected: rangeReceivedArr{},
	},
	{
		name: "single entries",
		mapIn: map[int]struct{}{
			1: struct{}{},
			3: struct{}{},
			5: struct{}{},
		},
		expected: rangeReceivedArr{
			{1, 1},
			{3, 3},
			{5, 5},
		},
	},
	{
		name: "ranges entries",
		mapIn: map[int]struct{}{
			1:  struct{}{},
			2:  struct{}{},
			3:  struct{}{},
			7:  struct{}{},
			9:  struct{}{},
			10: struct{}{},
			11: struct{}{},
			14: struct{}{},
			15: struct{}{},
			16: struct{}{},
			17: struct{}{},
			18: struct{}{},
			19: struct{}{},
		},
		expected: rangeReceivedArr{
			{1, 3},
			{7, 7},
			{9, 11},
			{14, 19},
		},
	},
}

// Test that the function that builds ranges in the receiver works.
func TestMapToRange(t *testing.T) {
	for _, test := range mapRangeTests {
		t.Run(test.name, func(t *testing.T) {
			seen := mapToRange(test.mapIn)
			if len(seen) != len(test.expected) {
				t.Fatalf("Length mismatch %d != %d.  Saw %v, expected %+v", len(seen), len(test.expected), seen, test.expected)
			}
			for i := range seen {
				if seen[i] != test.expected[i] {
					t.Fatalf("Element %d mismatch.  Saw %+v, expected %+v", i, seen[i], test.expected[i])
				}
			}
		})
	}
}
