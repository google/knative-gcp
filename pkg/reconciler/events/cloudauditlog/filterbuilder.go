package cloudauditlog

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

type FilterBuilder struct {
	serviceName  string
	methodName   string
	resourceName string
}

func (fb *FilterBuilder) SetServiceName(serviceName string) {
	fb.serviceName = serviceName
}

func (fb *FilterBuilder) SetMethodName(methodName string) {
	fb.methodName = methodName
}

func (fb *FilterBuilder) SetResourceName(resourceName string) {
	fb.resourceName = resourceName
}

func (fb *FilterBuilder) GetFilterQuery() string {
	var filters []string
	if fb.methodName != "" {
		filters = append(filters, fmt.Sprintf("%s=%q", methodKey, fb.methodName))
	}

	if fb.serviceName != "" {
		filters = append(filters, fmt.Sprintf("%s=%q", serviceKey, fb.serviceName))
	}

	if fb.resourceName != "" {
		filters = append(filters, fmt.Sprintf("%s=%q", resourceKey, fb.resourceName))
	}

	filters = append(filters, fmt.Sprintf("%s=%q", typeKey, typeValue))
	filter := strings.Join(filters, " AND ")
	return filter
}
