/*
Copyright 2019 Google LLC.

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

// Package resources contains helpers for audit log source resources.
package resources

import (
	"fmt"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	"github.com/google/knative-gcp/pkg/utils/naming"
)

// GenerateTopicName generates a topic name for the audit log
// source. This refers to the underlying Pub/Sub topic, and not our
// Topic resource.
func GenerateTopicName(s *v1.CloudAuditLogsSource) string {
	return naming.TruncatedPubsubResourceName("cre-src", s.Namespace, s.Name, s.UID)
}

// Generates the resource name for the topic used by an CloudAuditLogsSource.
func GenerateTopicResourceName(s *v1.CloudAuditLogsSource) string {
	return fmt.Sprintf("pubsub.googleapis.com/projects/%s/topics/%s", s.Status.ProjectID, s.Status.TopicID)
}

// GenerateSinkName generates a Stackdriver sink resource name for an
// CloudAuditLogsSource.
func GenerateSinkName(s *v1.CloudAuditLogsSource) string {
	return naming.TruncatedLoggingSinkResourceName("cre-src", s.Namespace, s.Name, s.UID)
}
