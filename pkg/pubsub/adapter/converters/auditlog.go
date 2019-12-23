/*
Copyright 2019 Google LLC

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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"go.uber.org/zap"
	auditpb "google.golang.org/genproto/googleapis/cloud/audit"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
	"knative.dev/pkg/logging"
)

const (
	AuditLogAdapterType = "google.auditlog"

	logEntrySchema = "type.googleapis.com/google.logging.v2.LogEntry"
	auditLogSchema = "type.googleapis.com/google.cloud.audit.AuditLog"
	loggingSource  = "logging.googleapis.com"
	EventType      = "AuditLog"
)

var (
	jsonpbUnmarshaller = jsonpb.Unmarshaler{
		AllowUnknownFields: true,
		AnyResolver:        resolver(resolveAnyUnknowns),
	}
	jsonpbMarshaler = jsonpb.Marshaler{}
)

type resolver func(turl string) (proto.Message, error)

func (r resolver) Resolve(turl string) (proto.Message, error) {
	return r(turl)
}

type UnknownMsg empty.Empty

func (m *UnknownMsg) ProtoMessage() {
	(*empty.Empty)(m).ProtoMessage()
}

func (m *UnknownMsg) Reset() {
	(*empty.Empty)(m).Reset()
}

func (m *UnknownMsg) String() string {
	return "Unknown message"
}

func resolveAnyUnknowns(typeURL string) (proto.Message, error) {
	// Only the part of typeUrl after the last slash is relevant.
	mname := typeURL
	if slash := strings.LastIndex(mname, "/"); slash >= 0 {
		mname = mname[slash+1:]
	}
	mt := proto.MessageType(mname)
	if mt == nil {
		return (*UnknownMsg)(&empty.Empty{}), nil
	}
	return reflect.New(mt.Elem()).Interface().(proto.Message), nil
}

func convertAuditLog(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	logger := logging.FromContext(ctx)
	if msg == nil {
		return nil, fmt.Errorf("nil pubsub message")
	}
	entry := logpb.LogEntry{}
	if err := jsonpbUnmarshaller.Unmarshal(bytes.NewReader(msg.Data), &entry); err != nil {
		return nil, fmt.Errorf("failed to decode LogEntry: %q", err)
	}
	logger = logger.With(zap.Any("LogEntry.LogName", entry.LogName), zap.Any("LogEntry.Resource", entry.Resource))
	logger.Info("Processing Stackdriver LogEntry.")
	tx := pubsubcontext.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	//TODO: Derive ID from entry
	event.SetID(tx.ID)
	if timestamp, err := ptypes.Timestamp(entry.Timestamp); err != nil {
		return nil, fmt.Errorf("invalid LogEntry timestamp: %q", err)
	} else {
		event.SetTime(timestamp)
	}
	switch payload := entry.Payload.(type) {
	case *logpb.LogEntry_ProtoPayload:
		var unpacked ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(payload.ProtoPayload, &unpacked); err != nil {
			return nil, fmt.Errorf("unrecognized proto payload: %q", err)
		}
		switch proto := unpacked.Message.(type) {
		case *auditpb.AuditLog:
			logger = logger.With(
				zap.Any("AuditLog.ServiceName", proto.ServiceName),
				zap.Any("AuditLog.ResourceName", proto.ResourceName),
				zap.Any("AuditLog.MethodName", proto.MethodName))
			logger.Info("Processing AuditLog.")
			event.SetSource(proto.ServiceName)
			event.SetSubject(proto.MethodName)
			event.SetType(EventType)
			event.SetDataSchema(auditLogSchema)
			event.SetDataContentType(cloudevents.ApplicationJSON)
			payload, err := json.Marshal(proto)
			if err != nil {
				return nil, fmt.Errorf("error marshalling AuditLog payload: %v", err)
			}
			event.SetData(payload)
		default:
			return nil, fmt.Errorf("unhandled proto payload type: %T", proto)
		}
	default:
		return nil, fmt.Errorf("non-AuditLog log entry")
	}
	logger.Debug("Created Stackdriver event: %v", event)
	return &event, nil
}
