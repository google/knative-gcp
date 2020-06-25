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

package v1

import (
	"crypto/md5"
	"fmt"
)

const (
	CloudAuditLogsLogWrittenEventType = "google.cloud.audit.log.v1.written"
	CloudAuditLogsEventDataSchema     = "https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/audit/v1/data.proto"
)

// CloudAuditLogsEventSource returns the Cloud Audit Logs CloudEvent source value.
// Format e.g. //cloudaudit.googleapis.com/projects/project-id/logs/[activity|data_access]
func CloudAuditLogsEventSource(parentResource, activity string) string {
	src := fmt.Sprintf("//cloudaudit.googleapis.com/%s", parentResource)
	if activity != "" {
		src = src + "/logs/" + activity
	}
	return src
}

// CloudAuditLogsEventID returns the Cloud Audit Logs CloudEvent id value.
func CloudAuditLogsEventID(id, logName, timestamp string) string {
	// Hash the concatenation of the three fields.
	return fmt.Sprintf("%x", md5.Sum([]byte(id+logName+timestamp)))
}

// CloudAuditLogsEventSubject returns the Cloud Audit Logs CloudEvent subject value.
func CloudAuditLogsEventSubject(serviceName, resourceName string) string {
	return fmt.Sprintf("%s/%s", serviceName, resourceName)
}
