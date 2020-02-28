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
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	auditpb "google.golang.org/genproto/googleapis/cloud/audit"
	logpb "google.golang.org/genproto/googleapis/logging/v2"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

const (
	CloudAuditLogsConverter = "com.google.cloud.auditlogs"

	logEntrySchema = "type.googleapis.com/google.logging.v2.LogEntry"

	parentResourcePattern = `^(:?projects|organizations|billingAccounts|folders)/[^/]+`

	serviceNameExtension  = "servicename"
	methodNameExtension   = "methodname"
	resourceNameExtension = "resourcename"
)

var (
	jsonpbUnmarshaller = jsonpb.Unmarshaler{
		AllowUnknownFields: true,
		AnyResolver:        resolver(resolveAnyUnknowns),
	}
	jsonpbMarshaler      = jsonpb.Marshaler{}
	parentResourceRegexp *regexp.Regexp
)

func init() {
	var err error
	if parentResourceRegexp, err = regexp.Compile(parentResourcePattern); err != nil {
		log.Fatal(err)
	}
}

// Resolver function type that can be used to resolve Any fields in a jsonpb.Unmarshaler.
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

// Resolves type URLs such as
// type.googleapis.com/google.profile.Person to a proto message
// type. Resolves unknown message types to empty.Empty.
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

func convertCloudAuditLogs(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil pubsub message")
	}
	entry := logpb.LogEntry{}
	if err := jsonpbUnmarshaller.Unmarshal(bytes.NewReader(msg.Data), &entry); err != nil {
		return nil, fmt.Errorf("failed to decode LogEntry: %w", err)
	}

	parentResource := parentResourceRegexp.FindString(entry.LogName)
	if parentResource == "" {
		return nil, fmt.Errorf("invalid LogName: %q", entry.LogName)
	}

	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(v1alpha1.CloudAuditLogsSourceEventID(entry.InsertId, entry.LogName, ptypes.TimestampString(entry.Timestamp)))
	if timestamp, err := ptypes.Timestamp(entry.Timestamp); err != nil {
		return nil, fmt.Errorf("invalid LogEntry timestamp: %w", err)
	} else {
		event.SetTime(timestamp)
	}
	event.SetData(msg.Data)
	event.SetDataSchema(logEntrySchema)
	event.SetDataContentType(cloudevents.ApplicationJSON)

	switch payload := entry.Payload.(type) {
	case *logpb.LogEntry_ProtoPayload:
		var unpacked ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(payload.ProtoPayload, &unpacked); err != nil {
			return nil, fmt.Errorf("unrecognized proto payload: %w", err)
		}
		switch proto := unpacked.Message.(type) {
		case *auditpb.AuditLog:
			event.SetType(v1alpha1.CloudAuditLogsSourceEvent)
			event.SetSource(v1alpha1.CloudAuditLogsSourceEventSource(proto.ServiceName, parentResource))
			event.SetSubject(proto.ResourceName)
			event.SetExtension(serviceNameExtension, proto.ServiceName)
			event.SetExtension(methodNameExtension, proto.MethodName)
			event.SetExtension(resourceNameExtension, proto.ResourceName)
		default:
			return nil, fmt.Errorf("unhandled proto payload type: %T", proto)
		}
	default:
		return nil, errors.New("non-AuditLog log entry")
	}
	return &event, nil
}
