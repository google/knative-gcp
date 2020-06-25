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

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	auditpb "google.golang.org/genproto/googleapis/cloud/audit"
	logpb "google.golang.org/genproto/googleapis/logging/v2"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

const (
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

// Log name, e.g. "organizations/1234567890/logs/cloudresourcemanager.googleapis.com%2Factivity"
func logActivity(logName string) string {
	parts := strings.Split(logName, "%2F")
	if len(parts) < 2 {
		return ""
	}
	// Could be "activity" or "data_access"
	return parts[1]
}

func convertCloudAuditLogs(ctx context.Context, msg *pubsub.Message) (*cev2.Event, error) {
	entry := logpb.LogEntry{}
	if err := jsonpbUnmarshaller.Unmarshal(bytes.NewReader(msg.Data), &entry); err != nil {
		return nil, fmt.Errorf("failed to decode LogEntry: %w", err)
	}

	parentResource := parentResourceRegexp.FindString(entry.LogName)
	if parentResource == "" {
		return nil, fmt.Errorf("invalid LogName: %q", entry.LogName)
	}
	logActivity := logActivity(entry.LogName)

	// Make a new event and convert the message payload.
	event := cev2.NewEvent(cev2.VersionV1)
	event.SetID(schemasv1.CloudAuditLogsEventID(entry.InsertId, entry.LogName, ptypes.TimestampString(entry.Timestamp)))
	if timestamp, err := ptypes.Timestamp(entry.Timestamp); err != nil {
		return nil, fmt.Errorf("invalid LogEntry timestamp: %w", err)
	} else {
		event.SetTime(timestamp)
	}
	event.SetType(schemasv1.CloudAuditLogsLogWrittenEventType)
	event.SetSource(schemasv1.CloudAuditLogsEventSource(parentResource, logActivity))
	event.SetDataSchema(schemasv1.CloudAuditLogsEventDataSchema)
	event.SetData(cev2.ApplicationJSON, msg.Data)

	switch payload := entry.Payload.(type) {
	case *logpb.LogEntry_ProtoPayload:
		var unpacked ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(payload.ProtoPayload, &unpacked); err != nil {
			return nil, fmt.Errorf("unrecognized proto payload: %w", err)
		}
		switch proto := unpacked.Message.(type) {
		case *auditpb.AuditLog:
			event.SetSubject(schemasv1.CloudAuditLogsEventSubject(proto.ServiceName, proto.ResourceName))
			// TODO: figure out if we want to keep these extensions.
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
