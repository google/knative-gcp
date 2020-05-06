package metrics

import (
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics/metricskey"
)

const ()

var (
	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	NamespaceNameKey = tag.MustNewKey(metricskey.LabelNamespaceName)

	BrokerNameKey        = tag.MustNewKey(metricskey.LabelBrokerName)
	EventTypeKey         = tag.MustNewKey(metricskey.LabelEventType)
	TriggerNameKey       = tag.MustNewKey(metricskey.LabelTriggerName)
	TriggerFilterTypeKey = tag.MustNewKey(metricskey.LabelFilterType)

	ResponseCodeKey      = tag.MustNewKey(metricskey.LabelResponseCode)
	ResponseCodeClassKey = tag.MustNewKey(metricskey.LabelResponseCodeClass)

	PodNameKey       = tag.MustNewKey(metricskey.PodName)
	ContainerNameKey = tag.MustNewKey(metricskey.ContainerName)
)
