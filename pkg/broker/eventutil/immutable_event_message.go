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

package eventutil

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
)

// ImmutableEventMessage wraps binding.EventMessage and sets binary encoding to avoid mutation of
// the underlying event.
type ImmutableEventMessage struct {
	*binding.EventMessage
}

func NewImmutableEventMessage(e *event.Event) ImmutableEventMessage {
	return ImmutableEventMessage{(*binding.EventMessage)(e)}
}

// ReadEncoding overrides the encoding used by EventMessage with EncodingBinary instead of
// EncodingEvent to avoid unnecessarily converting the message to an event.
func (m ImmutableEventMessage) ReadEncoding() binding.Encoding {
	return binding.EncodingBinary
}
