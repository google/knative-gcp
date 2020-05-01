// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/monitoring/v3/mutation_record.proto

package monitoring

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Describes a change made to a configuration.
type MutationRecord struct {
	// When the change occurred.
	MutateTime *timestamp.Timestamp `protobuf:"bytes,1,opt,name=mutate_time,json=mutateTime,proto3" json:"mutate_time,omitempty"`
	// The email address of the user making the change.
	MutatedBy            string   `protobuf:"bytes,2,opt,name=mutated_by,json=mutatedBy,proto3" json:"mutated_by,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MutationRecord) Reset()         { *m = MutationRecord{} }
func (m *MutationRecord) String() string { return proto.CompactTextString(m) }
func (*MutationRecord) ProtoMessage()    {}
func (*MutationRecord) Descriptor() ([]byte, []int) {
	return fileDescriptor_83c24e690bdb9101, []int{0}
}

func (m *MutationRecord) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MutationRecord.Unmarshal(m, b)
}
func (m *MutationRecord) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MutationRecord.Marshal(b, m, deterministic)
}
func (m *MutationRecord) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MutationRecord.Merge(m, src)
}
func (m *MutationRecord) XXX_Size() int {
	return xxx_messageInfo_MutationRecord.Size(m)
}
func (m *MutationRecord) XXX_DiscardUnknown() {
	xxx_messageInfo_MutationRecord.DiscardUnknown(m)
}

var xxx_messageInfo_MutationRecord proto.InternalMessageInfo

func (m *MutationRecord) GetMutateTime() *timestamp.Timestamp {
	if m != nil {
		return m.MutateTime
	}
	return nil
}

func (m *MutationRecord) GetMutatedBy() string {
	if m != nil {
		return m.MutatedBy
	}
	return ""
}

func init() {
	proto.RegisterType((*MutationRecord)(nil), "google.monitoring.v3.MutationRecord")
}

func init() {
	proto.RegisterFile("google/monitoring/v3/mutation_record.proto", fileDescriptor_83c24e690bdb9101)
}

var fileDescriptor_83c24e690bdb9101 = []byte{
	// 267 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x90, 0x31, 0x6a, 0xc3, 0x30,
	0x14, 0x86, 0x71, 0x86, 0x42, 0x14, 0xe8, 0xe0, 0x76, 0x30, 0x86, 0xd0, 0xd0, 0x29, 0x74, 0x90,
	0xa0, 0xda, 0x14, 0xe8, 0xe0, 0x0e, 0x9d, 0x02, 0xc1, 0x14, 0x0f, 0xc5, 0x60, 0xec, 0xd8, 0x15,
	0x02, 0x4b, 0xcf, 0x28, 0x72, 0x20, 0x57, 0xea, 0x51, 0xda, 0x9b, 0xf4, 0x14, 0xc5, 0x92, 0x8c,
	0x08, 0x74, 0x7c, 0xef, 0xfb, 0xf5, 0xbd, 0x1f, 0xa1, 0x27, 0x0e, 0xc0, 0xfb, 0x8e, 0x48, 0x50,
	0xc2, 0x80, 0x16, 0x8a, 0x93, 0x33, 0x25, 0x72, 0x34, 0xb5, 0x11, 0xa0, 0x2a, 0xdd, 0x1d, 0x41,
	0xb7, 0x78, 0xd0, 0x60, 0x20, 0xbe, 0x77, 0x59, 0x1c, 0xb2, 0xf8, 0x4c, 0xd3, 0x07, 0x6f, 0xb0,
	0x99, 0x66, 0xfc, 0x24, 0x46, 0xc8, 0xee, 0x64, 0x6a, 0x39, 0xb8, 0x67, 0x8f, 0x3d, 0xba, 0xdd,
	0x7b, 0x5f, 0x6e, 0x75, 0xf1, 0x0e, 0xad, 0xec, 0x85, 0xae, 0x9a, 0xb2, 0x49, 0xb4, 0x89, 0xb6,
	0xab, 0xe7, 0x14, 0x7b, 0xfd, 0x2c, 0xc2, 0xef, 0xb3, 0x28, 0x47, 0x2e, 0x3e, 0x2d, 0xe2, 0x35,
	0xf2, 0x53, 0x5b, 0x35, 0x97, 0x64, 0xb1, 0x89, 0xb6, 0xcb, 0x7c, 0xe9, 0x37, 0xd9, 0x25, 0xfb,
	0x89, 0x50, 0x72, 0x04, 0x89, 0xff, 0xeb, 0x9a, 0xdd, 0x5d, 0x17, 0x39, 0x4c, 0x97, 0x0e, 0xd1,
	0xc7, 0x8b, 0x0f, 0x73, 0xe8, 0x6b, 0xc5, 0x31, 0x68, 0x4e, 0x78, 0xa7, 0x6c, 0x0f, 0xe2, 0x50,
	0x3d, 0x88, 0xd3, 0xf5, 0x1f, 0xed, 0xc2, 0xf4, 0xb5, 0x48, 0xdf, 0x9c, 0xe0, 0xb5, 0x87, 0xb1,
	0xc5, 0xfb, 0x70, 0xb3, 0xa0, 0xdf, 0x33, 0x2c, 0x2d, 0x2c, 0x03, 0x2c, 0x0b, 0xfa, 0xbb, 0x58,
	0x3b, 0xc8, 0x98, 0xa5, 0x8c, 0x05, 0xcc, 0x58, 0x41, 0x9b, 0x1b, 0x5b, 0x82, 0xfe, 0x05, 0x00,
	0x00, 0xff, 0xff, 0xda, 0xe2, 0xe7, 0xb6, 0xa7, 0x01, 0x00, 0x00,
}
