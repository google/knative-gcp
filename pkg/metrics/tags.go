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

package metrics

import (
	"strings"

	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics/metricskey"
)

const (
	defaultEventType  = "custom"
	labelResourceKind = "resource_kind"
	labelResourceName = "resource_name"
)

type PodName string
type ContainerName string

var (
	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	NamespaceNameKey = tag.MustNewKey(metricskey.LabelNamespaceName)

	BrokerNameKey        = tag.MustNewKey(metricskey.LabelBrokerName)
	EventTypeKey         = tag.MustNewKey(metricskey.LabelEventType)
	ResourceKindKey      = tag.MustNewKey(labelResourceKind)
	ResourceNameKey      = tag.MustNewKey(labelResourceName)
	TriggerNameKey       = tag.MustNewKey(metricskey.LabelTriggerName)
	TriggerFilterTypeKey = tag.MustNewKey(metricskey.LabelFilterType)

	ResponseCodeKey      = tag.MustNewKey(metricskey.LabelResponseCode)
	ResponseCodeClassKey = tag.MustNewKey(metricskey.LabelResponseCodeClass)

	PodNameKey       = tag.MustNewKey(metricskey.PodName)
	ContainerNameKey = tag.MustNewKey(metricskey.ContainerName)
)

var (
	allowedEventTypes = map[string]struct{}{
		"e2e-dummy-event-type":              {},
		"e2e-testing-resp-event-type-dummy": {},
	}
)

// Stackdriver has a limit on the cardinality of fields on a metric, and event types can be any custom string. We're
// reducing the allowable values on this field to be limited to GCP event types and defaulting everything else to
// "custom".
func EventTypeMetricValue(eventType string) string {
	if strings.HasPrefix(eventType, "google.cloud") {
		return eventType
	}
	if _, contains := allowedEventTypes[eventType]; contains {
		return eventType
	}

	return defaultEventType
}
