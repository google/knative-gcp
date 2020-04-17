//
//Copyright 2020 Google LLC
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.21.0
// 	protoc        v3.8.0
// source: pkg/broker/config/targets.proto

package config

import (
	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

// The state of the object.
// We may add additional intermediate states if needed.
type State int32

const (
	State_UNKNOWN State = 0
	State_READY   State = 1
)

// Enum value maps for State.
var (
	State_name = map[int32]string{
		0: "UNKNOWN",
		1: "READY",
	}
	State_value = map[string]int32{
		"UNKNOWN": 0,
		"READY":   1,
	}
)

func (x State) Enum() *State {
	p := new(State)
	*p = x
	return p
}

func (x State) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (State) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_broker_config_targets_proto_enumTypes[0].Descriptor()
}

func (State) Type() protoreflect.EnumType {
	return &file_pkg_broker_config_targets_proto_enumTypes[0]
}

func (x State) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use State.Descriptor instead.
func (State) EnumDescriptor() ([]byte, []int) {
	return file_pkg_broker_config_targets_proto_rawDescGZIP(), []int{0}
}

// A pubsub "queue".
type Queue struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Topic        string `protobuf:"bytes,1,opt,name=topic,proto3" json:"topic,omitempty"`
	Subscription string `protobuf:"bytes,2,opt,name=subscription,proto3" json:"subscription,omitempty"`
}

func (x *Queue) Reset() {
	*x = Queue{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_broker_config_targets_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Queue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Queue) ProtoMessage() {}

func (x *Queue) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_broker_config_targets_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Queue.ProtoReflect.Descriptor instead.
func (*Queue) Descriptor() ([]byte, []int) {
	return file_pkg_broker_config_targets_proto_rawDescGZIP(), []int{0}
}

func (x *Queue) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

func (x *Queue) GetSubscription() string {
	if x != nil {
		return x.Subscription
	}
	return ""
}

// Represents a broker.
type Broker struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The id of the object. E.g. UID of the resource.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// The name of the object.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// The namespace of the object.
	Namespace string `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	// The broker address.
	// Will we have more than one address?
	Address string `protobuf:"bytes,4,opt,name=address,proto3" json:"address,omitempty"`
	// The decouple queue for the broker.
	DecoupleQueue *Queue `protobuf:"bytes,5,opt,name=decouple_queue,json=decoupleQueue,proto3" json:"decouple_queue,omitempty"`
	// All targets of the broker.
	Targets map[string]*Target `protobuf:"bytes,6,rep,name=targets,proto3" json:"targets,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// The broker state.
	State State `protobuf:"varint,7,opt,name=state,proto3,enum=config.State" json:"state,omitempty"`
}

func (x *Broker) Reset() {
	*x = Broker{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_broker_config_targets_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Broker) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Broker) ProtoMessage() {}

func (x *Broker) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_broker_config_targets_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Broker.ProtoReflect.Descriptor instead.
func (*Broker) Descriptor() ([]byte, []int) {
	return file_pkg_broker_config_targets_proto_rawDescGZIP(), []int{1}
}

func (x *Broker) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Broker) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Broker) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *Broker) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Broker) GetDecoupleQueue() *Queue {
	if x != nil {
		return x.DecoupleQueue
	}
	return nil
}

func (x *Broker) GetTargets() map[string]*Target {
	if x != nil {
		return x.Targets
	}
	return nil
}

func (x *Broker) GetState() State {
	if x != nil {
		return x.State
	}
	return State_UNKNOWN
}

// Target defines the config schema for a broker subscription target.
type Target struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The id of the object. E.g. UID of the resource.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// The name of the object.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// The namespace of the object.
	Namespace string `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	// The broker name that the trigger is referencing.
	Broker string `protobuf:"bytes,4,opt,name=broker,proto3" json:"broker,omitempty"`
	// The resolved subscriber URI of the target.
	Address string `protobuf:"bytes,5,opt,name=address,proto3" json:"address,omitempty"`
	// Optional filters from the trigger.
	FilterAttributes map[string]string `protobuf:"bytes,6,rep,name=filter_attributes,json=filterAttributes,proto3" json:"filter_attributes,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// The retry queue for the target.
	RetryQueue *Queue `protobuf:"bytes,7,opt,name=retry_queue,json=retryQueue,proto3" json:"retry_queue,omitempty"`
	// The target state.
	State State `protobuf:"varint,8,opt,name=state,proto3,enum=config.State" json:"state,omitempty"`
}

func (x *Target) Reset() {
	*x = Target{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_broker_config_targets_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Target) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Target) ProtoMessage() {}

func (x *Target) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_broker_config_targets_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Target.ProtoReflect.Descriptor instead.
func (*Target) Descriptor() ([]byte, []int) {
	return file_pkg_broker_config_targets_proto_rawDescGZIP(), []int{2}
}

func (x *Target) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Target) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Target) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *Target) GetBroker() string {
	if x != nil {
		return x.Broker
	}
	return ""
}

func (x *Target) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Target) GetFilterAttributes() map[string]string {
	if x != nil {
		return x.FilterAttributes
	}
	return nil
}

func (x *Target) GetRetryQueue() *Queue {
	if x != nil {
		return x.RetryQueue
	}
	return nil
}

func (x *Target) GetState() State {
	if x != nil {
		return x.State
	}
	return State_UNKNOWN
}

// TargetsConfig is the collection of all Targets.
type TargetsConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Keybed by broker namespace/name.
	Brokers map[string]*Broker `protobuf:"bytes,1,rep,name=brokers,proto3" json:"brokers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *TargetsConfig) Reset() {
	*x = TargetsConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_broker_config_targets_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TargetsConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TargetsConfig) ProtoMessage() {}

func (x *TargetsConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_broker_config_targets_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TargetsConfig.ProtoReflect.Descriptor instead.
func (*TargetsConfig) Descriptor() ([]byte, []int) {
	return file_pkg_broker_config_targets_proto_rawDescGZIP(), []int{3}
}

func (x *TargetsConfig) GetBrokers() map[string]*Broker {
	if x != nil {
		return x.Brokers
	}
	return nil
}

var File_pkg_broker_config_targets_proto protoreflect.FileDescriptor

var file_pkg_broker_config_targets_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x70, 0x6b, 0x67, 0x2f, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x2f, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x2f, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x22, 0x41, 0x0a, 0x05, 0x51, 0x75, 0x65,
	0x75, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x12, 0x22, 0x0a, 0x0c, 0x73, 0x75, 0x62, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c,
	0x73, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0xc2, 0x02, 0x0a,
	0x06, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x6e,
	0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09,
	0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64,
	0x72, 0x65, 0x73, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x12, 0x34, 0x0a, 0x0e, 0x64, 0x65, 0x63, 0x6f, 0x75, 0x70, 0x6c, 0x65, 0x5f,
	0x71, 0x75, 0x65, 0x75, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x2e, 0x51, 0x75, 0x65, 0x75, 0x65, 0x52, 0x0d, 0x64, 0x65, 0x63, 0x6f,
	0x75, 0x70, 0x6c, 0x65, 0x51, 0x75, 0x65, 0x75, 0x65, 0x12, 0x35, 0x0a, 0x07, 0x74, 0x61, 0x72,
	0x67, 0x65, 0x74, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x2e, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x2e, 0x54, 0x61, 0x72, 0x67, 0x65,
	0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x73,
	0x12, 0x23, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x0d, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x05,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x1a, 0x4a, 0x0a, 0x0c, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74, 0x73,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x24, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e,
	0x54, 0x61, 0x72, 0x67, 0x65, 0x74, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38,
	0x01, 0x22, 0xe9, 0x02, 0x0a, 0x06, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74, 0x12, 0x0e, 0x0a, 0x02,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x12, 0x51, 0x0a, 0x11, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x5f, 0x61, 0x74, 0x74, 0x72, 0x69,
	0x62, 0x75, 0x74, 0x65, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x2e, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74, 0x2e, 0x46, 0x69, 0x6c, 0x74,
	0x65, 0x72, 0x41, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x52, 0x10, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x41, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75,
	0x74, 0x65, 0x73, 0x12, 0x2e, 0x0a, 0x0b, 0x72, 0x65, 0x74, 0x72, 0x79, 0x5f, 0x71, 0x75, 0x65,
	0x75, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x51, 0x75, 0x65, 0x75, 0x65, 0x52, 0x0a, 0x72, 0x65, 0x74, 0x72, 0x79, 0x51, 0x75,
	0x65, 0x75, 0x65, 0x12, 0x23, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x08, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x0d, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x53, 0x74, 0x61, 0x74,
	0x65, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x1a, 0x43, 0x0a, 0x15, 0x46, 0x69, 0x6c, 0x74,
	0x65, 0x72, 0x41, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03,
	0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x99, 0x01,
	0x0a, 0x0d, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74, 0x73, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12,
	0x3c, 0x0a, 0x07, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x22, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x54, 0x61, 0x72, 0x67, 0x65, 0x74,
	0x73, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73, 0x1a, 0x4a, 0x0a,
	0x0c, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a,
	0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12,
	0x24, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e,
	0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x42, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x2a, 0x1f, 0x0a, 0x05, 0x53, 0x74, 0x61,
	0x74, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12,
	0x09, 0x0a, 0x05, 0x52, 0x45, 0x41, 0x44, 0x59, 0x10, 0x01, 0x42, 0x31, 0x5a, 0x2f, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x6b, 0x6e, 0x61, 0x74, 0x69, 0x76, 0x65, 0x2d, 0x67, 0x63, 0x70, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x62, 0x72, 0x6f, 0x6b, 0x65, 0x72, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_broker_config_targets_proto_rawDescOnce sync.Once
	file_pkg_broker_config_targets_proto_rawDescData = file_pkg_broker_config_targets_proto_rawDesc
)

func file_pkg_broker_config_targets_proto_rawDescGZIP() []byte {
	file_pkg_broker_config_targets_proto_rawDescOnce.Do(func() {
		file_pkg_broker_config_targets_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_broker_config_targets_proto_rawDescData)
	})
	return file_pkg_broker_config_targets_proto_rawDescData
}

var file_pkg_broker_config_targets_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pkg_broker_config_targets_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_pkg_broker_config_targets_proto_goTypes = []interface{}{
	(State)(0),            // 0: config.State
	(*Queue)(nil),         // 1: config.Queue
	(*Broker)(nil),        // 2: config.Broker
	(*Target)(nil),        // 3: config.Target
	(*TargetsConfig)(nil), // 4: config.TargetsConfig
	nil,                   // 5: config.Broker.TargetsEntry
	nil,                   // 6: config.Target.FilterAttributesEntry
	nil,                   // 7: config.TargetsConfig.BrokersEntry
}
var file_pkg_broker_config_targets_proto_depIdxs = []int32{
	1, // 0: config.Broker.decouple_queue:type_name -> config.Queue
	5, // 1: config.Broker.targets:type_name -> config.Broker.TargetsEntry
	0, // 2: config.Broker.state:type_name -> config.State
	6, // 3: config.Target.filter_attributes:type_name -> config.Target.FilterAttributesEntry
	1, // 4: config.Target.retry_queue:type_name -> config.Queue
	0, // 5: config.Target.state:type_name -> config.State
	7, // 6: config.TargetsConfig.brokers:type_name -> config.TargetsConfig.BrokersEntry
	3, // 7: config.Broker.TargetsEntry.value:type_name -> config.Target
	2, // 8: config.TargetsConfig.BrokersEntry.value:type_name -> config.Broker
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_pkg_broker_config_targets_proto_init() }
func file_pkg_broker_config_targets_proto_init() {
	if File_pkg_broker_config_targets_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_broker_config_targets_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Queue); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_broker_config_targets_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Broker); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_broker_config_targets_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Target); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_broker_config_targets_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TargetsConfig); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_broker_config_targets_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_broker_config_targets_proto_goTypes,
		DependencyIndexes: file_pkg_broker_config_targets_proto_depIdxs,
		EnumInfos:         file_pkg_broker_config_targets_proto_enumTypes,
		MessageInfos:      file_pkg_broker_config_targets_proto_msgTypes,
	}.Build()
	File_pkg_broker_config_targets_proto = out.File
	file_pkg_broker_config_targets_proto_rawDesc = nil
	file_pkg_broker_config_targets_proto_goTypes = nil
	file_pkg_broker_config_targets_proto_depIdxs = nil
}
