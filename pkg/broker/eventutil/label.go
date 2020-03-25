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
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// labelPrefix is the prefix for label keys.
	labelPrefix = "kgcp"
)

// LabeledEvent is a wrapper of a cloudevent
// that allows labeling the event.
type LabeledEvent struct {
	event *cloudevents.Event
}

// NewLabeledEvent creates a new LabeledEvent.
func NewLabeledEvent(e *cloudevents.Event) *LabeledEvent {
	return &LabeledEvent{event: e}
}

// WithLabel attaches a label to the event as an extension.
func (le *LabeledEvent) WithLabel(key, value string) *LabeledEvent {
	if value == "" {
		le.event.SetExtension(labelPrefix+key, nil)
	} else {
		le.event.SetExtension(labelPrefix+key, value)
	}
	return le
}

// GetLabels gets all the labels as a map.
func (le *LabeledEvent) GetLabels() map[string]string {
	m := make(map[string]string)
	exts := le.event.Extensions()
	for k, v := range exts {
		if strings.HasPrefix(strings.ToLower(k), labelPrefix) {
			m[strings.TrimPrefix(strings.ToLower(k), labelPrefix)] = v.(string)
		}
	}
	return m
}

// Event returns the LabeledEvent as a cloudevent.
func (le *LabeledEvent) Event() *cloudevents.Event {
	return le.event
}

// Delabeled returns the cloudevent without labels.
func (le *LabeledEvent) Delabeled() *cloudevents.Event {
	e := le.event.Clone()
	for k := range e.Extensions() {
		if strings.HasPrefix(strings.ToLower(k), labelPrefix) {
			// Set to nil to delete that extension.
			e.SetExtension(k, nil)
		}
	}
	return &e
}
