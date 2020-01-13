package resources

import (
	"fmt"
	"strings"
)

const (
	keyPrefix   = "protoPayload"
	methodKey   = keyPrefix + ".methodName"
	serviceKey  = keyPrefix + ".serviceName"
	resourceKey = keyPrefix + ".resourceName"
	typeKey     = keyPrefix + ".\x22@type\x22"
	typeValue   = "type.googleapis.com/google.cloud.audit.AuditLog"
)

// Stackdriver query builder for querying audit logs. Currently
// supports querying by the AuditLog serviceName, methodName, and
// resourceName.
type FilterBuilder struct {
	serviceName  string
	methodName   string
	resourceName string
}

func (fb *FilterBuilder) WithServiceName(serviceName string) *FilterBuilder {
	fb.serviceName = serviceName
	return fb
}

func (fb *FilterBuilder) WithMethodName(methodName string) *FilterBuilder {
	fb.methodName = methodName
	return fb
}

func (fb *FilterBuilder) WithResourceName(resourceName string) *FilterBuilder {
	fb.resourceName = resourceName
	return fb
}

func (fb *FilterBuilder) GetFilterQuery() string {
	var filters []string
	if fb.methodName != "" {
		filters = append(filters, filter{methodKey, fb.methodName}.String())
	}

	if fb.serviceName != "" {
		filters = append(filters, filter{serviceKey, fb.serviceName}.String())
	}

	if fb.resourceName != "" {
		filters = append(filters, filter{resourceKey, fb.resourceName}.String())
	}

	filters = append(filters, filter{typeKey, typeValue}.String())
	filter := strings.Join(filters, " AND ")
	return filter
}

type filter struct {
	key   string
	value string
}

func (f filter) String() string {
	return fmt.Sprintf("%s=%q", f.key, f.value)
}
