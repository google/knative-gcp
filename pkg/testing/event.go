/*
Copyright 2020 Google LLC.

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

package testing

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
)

var (
	BenchmarkEventSizes []int = []int{0, 1000, 100000}
)

func NewTestEvent(t testing.TB, size int) *event.Event {
	event := event.New()
	event.SetID("id")
	event.SetSource("source")
	event.SetSubject("subject")
	event.SetType("type")
	event.SetTime(time.Now())
	if size > 0 {
		data := make([]byte, size)
		_, err := rand.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		event.DataEncoded = data
		return &event
	}
	return &event
}
