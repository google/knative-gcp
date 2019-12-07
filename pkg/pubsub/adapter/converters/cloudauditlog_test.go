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

	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	auditpb "google.golang.org/genproto/googleapis/cloud/audit"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

var (
	tctx = pubsubcontext.TransportContext{
		ID:      "test-id",
		Project: "test-project",
		Topic:   "test-topic",
	}
)

func TestConvertCloudAuditLog(t *testing.T) {
	ctx := pubsubcontext.WithTransportContext(context.Background(), tctx)
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
		Timestamp: &timestamp.Timestamp{
			Seconds: 12345,
		},
		Payload: &logpb.LogEntry_ProtoPayload{
			ProtoPayload: payload,
		},
	}
	var buf bytes.Buffer
	if err := new(jsonpb.Marshaler).Marshal(&buf, &logEntry); err != nil {
		t.Fatalf("Failed to marshal AuditLog pb: %v", err)
	}
	msg := cepubsub.Message{
		Data: buf.Bytes(),
	}

	e, err := convertCloudAuditLog(ctx, &msg, "")

	if err != nil {
		t.Errorf("conversion failed: %v", err)
	}
	if e.Source() != "test-service-name" {
		t.Errorf("Source '%s' != 'test-service-name'", e.Source())
	}
	if e.Type() != "AuditLog" {
		t.Errorf("Type '%s' != 'AuditLog'", e.Type())
	}
	if e.Subject() != "test-method-name" {
		t.Errorf("Subject '%s' != 'test-method-name'", e.Subject())
	}
	t.Logf("Data: %s", e.Data)
	var actualAuditLog auditpb.AuditLog
	e.DataAs(&actualAuditLog)
	if actualAuditLog.ResourceName != "test-resource-name" {
		t.Errorf("AuditLog.ResourceName '%s' != 'test-resource-name'", actualAuditLog.ResourceName)
	}
}
