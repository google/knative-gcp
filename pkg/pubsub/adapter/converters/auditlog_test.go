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

package converters

import (
	"bytes"
	"context"
	"testing"
	"time"

	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	auditpb "google.golang.org/genproto/googleapis/cloud/audit"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

const (
	insertID = "test-insert-id"
	logName  = "projects/test-project/test-log-name"
	testTs   = "2006-01-02T15:04:05Z"
)

func TestConvertAuditLog(t *testing.T) {
	auditLog := auditpb.AuditLog{
		ServiceName:  "test-service-name",
		MethodName:   "test-method-name",
		ResourceName: "test-resource-name",
	}
	payload, err := ptypes.MarshalAny(&auditLog)
	if err != nil {
		t.Fatalf("Failed to marshal proto payload: %v", err)
	}
	logEntry := logpb.LogEntry{
		InsertId: insertID,
		LogName:  logName,
		Timestamp: &timestamp.Timestamp{
			Seconds: 12345,
		},
		Payload: &logpb.LogEntry_ProtoPayload{
			ProtoPayload: payload,
		},
	}
	testTime, err := time.Parse(time.RFC3339, testTs)
	if err != nil {
		t.Fatalf("Unable to parse test timestamp: %q", err)
	}
	if ts, err := ptypes.TimestampProto(testTime); err != nil {
		t.Fatalf("Invalid test timestamp: %q", err)
	} else {
		logEntry.Timestamp = ts
	}
	var buf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&buf, &logEntry); err != nil {
		t.Fatalf("Failed to marshal AuditLog pb: %v", err)
	}
	msg := cepubsub.Message{
		Data: buf.Bytes(),
	}

	e, err := Convert(context.Background(), &msg, "", AuditLogAdapterType)

	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}
	if e.ID() != insertID+logName+testTs {
		t.Errorf("ID '%s' != '%s%s%s'", e.ID(), insertID, logName, testTs)
	}
	if !e.Time().Equal(testTime) {
		t.Errorf("Time '%v' != '%v'", e.Time(), testTime)
	}
	if want := "test-service-name/projects/test-project"; e.Source() != want {
		t.Errorf("Source %q != %q", e.Source(), want)
	}
	if e.Type() != "com.google.cloud.auditlog.event" {
		t.Errorf(`Type %q != "com.google.cloud.auditlog.event"`, e.Type())
	}
	if want := "test-service-name/test-resource-name"; e.Subject() != want {
		t.Errorf("Subject %q != %q", e.Subject(), want)
	}
	if data, err := e.DataBytes(); err != nil {
		t.Errorf("Unable to get event data: %q", err)
	} else {
		var actualLogEntry logpb.LogEntry
		if err = jsonpb.Unmarshal(bytes.NewReader(data), &actualLogEntry); err != nil {
			t.Errorf("Unable to unmarshal event data to LogEntry: %q", err)
		} else {
			if diff := cmp.Diff(logEntry, actualLogEntry); diff != "" {
				t.Errorf("unexpected LogEntry (-want, +got) = %v", diff)
			}
		}
	}
	wantExtensions := map[string]interface{}{
		"servicename":  "test-service-name",
		"methodname":   "test-method-name",
		"resourcename": "test-resource-name",
	}
	if diff := cmp.Diff(wantExtensions, e.Extensions()); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestConvertTextPayload(t *testing.T) {
	logEntry := logpb.LogEntry{
		InsertId: insertID,
		LogName:  logName,
		Timestamp: &timestamp.Timestamp{
			Seconds: 12345,
		},
		Payload: &logpb.LogEntry_TextPayload{
			TextPayload: "test payload",
		},
	}
	testTime, err := time.Parse(time.RFC3339, testTs)
	if err != nil {
		t.Fatalf("Unable to parse test timestamp: %q", err)
	}
	if ts, err := ptypes.TimestampProto(testTime); err != nil {
		t.Fatalf("Invalid test timestamp: %q", err)
	} else {
		logEntry.Timestamp = ts
	}
	var buf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&buf, &logEntry); err != nil {
		t.Fatalf("Failed to marshal AuditLog pb: %v", err)
	}
	msg := cepubsub.Message{
		Data: buf.Bytes(),
	}

	_, err = Convert(context.Background(), &msg, "", AuditLogAdapterType)

	if err == nil {
		t.Errorf("Expected error when converting non-AuditLog LogEntry.")
	}
}
