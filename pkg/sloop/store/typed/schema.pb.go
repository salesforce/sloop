// Code generated by protoc-gen-go. DO NOT EDIT.
// source: schema.proto

package typed

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	math "math"
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

type KubeWatchResult_WatchType int32

const (
	KubeWatchResult_ADD    KubeWatchResult_WatchType = 0
	KubeWatchResult_UPDATE KubeWatchResult_WatchType = 1
	KubeWatchResult_DELETE KubeWatchResult_WatchType = 2
)

var KubeWatchResult_WatchType_name = map[int32]string{
	0: "ADD",
	1: "UPDATE",
	2: "DELETE",
}

var KubeWatchResult_WatchType_value = map[string]int32{
	"ADD":    0,
	"UPDATE": 1,
	"DELETE": 2,
}

func (x KubeWatchResult_WatchType) String() string {
	return proto.EnumName(KubeWatchResult_WatchType_name, int32(x))
}

func (KubeWatchResult_WatchType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_1c5fb4d8cc22d66a, []int{0, 0}
}

type KubeWatchResult struct {
	Timestamp            *timestamp.Timestamp      `protobuf:"bytes,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Kind                 string                    `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind,omitempty"`
	WatchType            KubeWatchResult_WatchType `protobuf:"varint,3,opt,name=watchType,proto3,enum=typed.KubeWatchResult_WatchType" json:"watchType,omitempty"`
	Payload              string                    `protobuf:"bytes,4,opt,name=payload,proto3" json:"payload,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                  `json:"-"`
	XXX_unrecognized     []byte                    `json:"-"`
	XXX_sizecache        int32                     `json:"-"`
}

func (m *KubeWatchResult) Reset()         { *m = KubeWatchResult{} }
func (m *KubeWatchResult) String() string { return proto.CompactTextString(m) }
func (*KubeWatchResult) ProtoMessage()    {}
func (*KubeWatchResult) Descriptor() ([]byte, []int) {
	return fileDescriptor_1c5fb4d8cc22d66a, []int{0}
}

func (m *KubeWatchResult) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_KubeWatchResult.Unmarshal(m, b)
}
func (m *KubeWatchResult) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_KubeWatchResult.Marshal(b, m, deterministic)
}
func (m *KubeWatchResult) XXX_Merge(src proto.Message) {
	xxx_messageInfo_KubeWatchResult.Merge(m, src)
}
func (m *KubeWatchResult) XXX_Size() int {
	return xxx_messageInfo_KubeWatchResult.Size(m)
}
func (m *KubeWatchResult) XXX_DiscardUnknown() {
	xxx_messageInfo_KubeWatchResult.DiscardUnknown(m)
}

var xxx_messageInfo_KubeWatchResult proto.InternalMessageInfo

func (m *KubeWatchResult) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *KubeWatchResult) GetKind() string {
	if m != nil {
		return m.Kind
	}
	return ""
}

func (m *KubeWatchResult) GetWatchType() KubeWatchResult_WatchType {
	if m != nil {
		return m.WatchType
	}
	return KubeWatchResult_ADD
}

func (m *KubeWatchResult) GetPayload() string {
	if m != nil {
		return m.Payload
	}
	return ""
}

// Enough information to draw a timeline and hierarchy
// Key: /<kind>/<namespace>/<name>/<uid>
type ResourceSummary struct {
	FirstSeen    *timestamp.Timestamp `protobuf:"bytes,1,opt,name=firstSeen,proto3" json:"firstSeen,omitempty"`
	LastSeen     *timestamp.Timestamp `protobuf:"bytes,2,opt,name=lastSeen,proto3" json:"lastSeen,omitempty"`
	CreateTime   *timestamp.Timestamp `protobuf:"bytes,3,opt,name=createTime,proto3" json:"createTime,omitempty"`
	DeletedAtEnd bool                 `protobuf:"varint,4,opt,name=deletedAtEnd,proto3" json:"deletedAtEnd,omitempty"`
	// List of relationships.  Direction does not matter.  Examples:
	// A Pod has a relationship to its namespace, ReplicaSet or StatefulSet, node
	// A ReplicaSet has a relationship to deployment and namespace
	// A node might have a relationship to a rack (maybe latery, as this is virtual)
	// We dont need relationships in both directions.  We can union them at query time
	// Uses same key format here as this overall table
	Relationships        []string `protobuf:"bytes,5,rep,name=relationships,proto3" json:"relationships,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ResourceSummary) Reset()         { *m = ResourceSummary{} }
func (m *ResourceSummary) String() string { return proto.CompactTextString(m) }
func (*ResourceSummary) ProtoMessage()    {}
func (*ResourceSummary) Descriptor() ([]byte, []int) {
	return fileDescriptor_1c5fb4d8cc22d66a, []int{1}
}

func (m *ResourceSummary) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ResourceSummary.Unmarshal(m, b)
}
func (m *ResourceSummary) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ResourceSummary.Marshal(b, m, deterministic)
}
func (m *ResourceSummary) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ResourceSummary.Merge(m, src)
}
func (m *ResourceSummary) XXX_Size() int {
	return xxx_messageInfo_ResourceSummary.Size(m)
}
func (m *ResourceSummary) XXX_DiscardUnknown() {
	xxx_messageInfo_ResourceSummary.DiscardUnknown(m)
}

var xxx_messageInfo_ResourceSummary proto.InternalMessageInfo

func (m *ResourceSummary) GetFirstSeen() *timestamp.Timestamp {
	if m != nil {
		return m.FirstSeen
	}
	return nil
}

func (m *ResourceSummary) GetLastSeen() *timestamp.Timestamp {
	if m != nil {
		return m.LastSeen
	}
	return nil
}

func (m *ResourceSummary) GetCreateTime() *timestamp.Timestamp {
	if m != nil {
		return m.CreateTime
	}
	return nil
}

func (m *ResourceSummary) GetDeletedAtEnd() bool {
	if m != nil {
		return m.DeletedAtEnd
	}
	return false
}

func (m *ResourceSummary) GetRelationships() []string {
	if m != nil {
		return m.Relationships
	}
	return nil
}

type EventCounts struct {
	MapReasonToCount     map[string]int32 `protobuf:"bytes,1,rep,name=mapReasonToCount,proto3" json:"mapReasonToCount,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *EventCounts) Reset()         { *m = EventCounts{} }
func (m *EventCounts) String() string { return proto.CompactTextString(m) }
func (*EventCounts) ProtoMessage()    {}
func (*EventCounts) Descriptor() ([]byte, []int) {
	return fileDescriptor_1c5fb4d8cc22d66a, []int{2}
}

func (m *EventCounts) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EventCounts.Unmarshal(m, b)
}
func (m *EventCounts) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EventCounts.Marshal(b, m, deterministic)
}
func (m *EventCounts) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EventCounts.Merge(m, src)
}
func (m *EventCounts) XXX_Size() int {
	return xxx_messageInfo_EventCounts.Size(m)
}
func (m *EventCounts) XXX_DiscardUnknown() {
	xxx_messageInfo_EventCounts.DiscardUnknown(m)
}

var xxx_messageInfo_EventCounts proto.InternalMessageInfo

func (m *EventCounts) GetMapReasonToCount() map[string]int32 {
	if m != nil {
		return m.MapReasonToCount
	}
	return nil
}

type ResourceEventCounts struct {
	MapMinToEvents       map[int64]*EventCounts `protobuf:"bytes,1,rep,name=mapMinToEvents,proto3" json:"mapMinToEvents,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *ResourceEventCounts) Reset()         { *m = ResourceEventCounts{} }
func (m *ResourceEventCounts) String() string { return proto.CompactTextString(m) }
func (*ResourceEventCounts) ProtoMessage()    {}
func (*ResourceEventCounts) Descriptor() ([]byte, []int) {
	return fileDescriptor_1c5fb4d8cc22d66a, []int{3}
}

func (m *ResourceEventCounts) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ResourceEventCounts.Unmarshal(m, b)
}
func (m *ResourceEventCounts) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ResourceEventCounts.Marshal(b, m, deterministic)
}
func (m *ResourceEventCounts) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ResourceEventCounts.Merge(m, src)
}
func (m *ResourceEventCounts) XXX_Size() int {
	return xxx_messageInfo_ResourceEventCounts.Size(m)
}
func (m *ResourceEventCounts) XXX_DiscardUnknown() {
	xxx_messageInfo_ResourceEventCounts.DiscardUnknown(m)
}

var xxx_messageInfo_ResourceEventCounts proto.InternalMessageInfo

func (m *ResourceEventCounts) GetMapMinToEvents() map[int64]*EventCounts {
	if m != nil {
		return m.MapMinToEvents
	}
	return nil
}

// Track when 'watch' occurred for a resource within partition
type WatchActivity struct {
	// List of timestamps where `watch` event did not contain changes from previous event
	NoChangeAt []int64 `protobuf:"varint,1,rep,packed,name=NoChangeAt,proto3" json:"NoChangeAt,omitempty"`
	// List of timestamps where 'watch' event contained a change from previous event
	ChangedAt            []int64  `protobuf:"varint,2,rep,packed,name=ChangedAt,proto3" json:"ChangedAt,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *WatchActivity) Reset()         { *m = WatchActivity{} }
func (m *WatchActivity) String() string { return proto.CompactTextString(m) }
func (*WatchActivity) ProtoMessage()    {}
func (*WatchActivity) Descriptor() ([]byte, []int) {
	return fileDescriptor_1c5fb4d8cc22d66a, []int{4}
}

func (m *WatchActivity) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_WatchActivity.Unmarshal(m, b)
}
func (m *WatchActivity) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_WatchActivity.Marshal(b, m, deterministic)
}
func (m *WatchActivity) XXX_Merge(src proto.Message) {
	xxx_messageInfo_WatchActivity.Merge(m, src)
}
func (m *WatchActivity) XXX_Size() int {
	return xxx_messageInfo_WatchActivity.Size(m)
}
func (m *WatchActivity) XXX_DiscardUnknown() {
	xxx_messageInfo_WatchActivity.DiscardUnknown(m)
}

var xxx_messageInfo_WatchActivity proto.InternalMessageInfo

func (m *WatchActivity) GetNoChangeAt() []int64 {
	if m != nil {
		return m.NoChangeAt
	}
	return nil
}

func (m *WatchActivity) GetChangedAt() []int64 {
	if m != nil {
		return m.ChangedAt
	}
	return nil
}

type Relationship struct {
	FirstSeen    *timestamp.Timestamp `protobuf:"bytes,1,opt,name=firstSeen,proto3" json:"firstSeen,omitempty"`
	LastSeen     *timestamp.Timestamp `protobuf:"bytes,2,opt,name=lastSeen,proto3" json:"lastSeen,omitempty"`
	CreateTime   *timestamp.Timestamp `protobuf:"bytes,3,opt,name=createTime,proto3" json:"createTime,omitempty"`
	DeletedAtEnd bool                 `protobuf:"varint,4,opt,name=deletedAtEnd,proto3" json:"deletedAtEnd,omitempty"`
	// List of relationships.  Direction does not matter.  Examples:
	// A Pod has a relationship to its ReplicaSet and node
	// A ReplicaSet has a relationship to deployment
	Relationships        []string `protobuf:"bytes,5,rep,name=relationships,proto3" json:"relationships,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

//func (r Relationship) Reset() {
//	panic("implement me")
//}
func (m *Relationship) Reset()         { *m = Relationship{} }

func (r Relationship) String() string {
	panic("implement me")
}

func (r Relationship) ProtoMessage() {
	panic("implement me")
}

func init() {
	proto.RegisterEnum("typed.KubeWatchResult_WatchType", KubeWatchResult_WatchType_name, KubeWatchResult_WatchType_value)
	proto.RegisterType((*KubeWatchResult)(nil), "typed.KubeWatchResult")
	proto.RegisterType((*ResourceSummary)(nil), "typed.ResourceSummary")
	proto.RegisterType((*EventCounts)(nil), "typed.EventCounts")
	proto.RegisterType((*Relationship)(nil), "typed.Relationship")
	proto.RegisterMapType((map[string]int32)(nil), "typed.EventCounts.MapReasonToCountEntry")
	proto.RegisterType((*ResourceEventCounts)(nil), "typed.ResourceEventCounts")
	proto.RegisterMapType((map[int64]*EventCounts)(nil), "typed.ResourceEventCounts.MapMinToEventsEntry")
	proto.RegisterType((*WatchActivity)(nil), "typed.WatchActivity")
}

func init() { proto.RegisterFile("schema.proto", fileDescriptor_1c5fb4d8cc22d66a) }

var fileDescriptor_1c5fb4d8cc22d66a = []byte{
	// 505 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x92, 0xcf, 0x6e, 0x9b, 0x40,
	0x10, 0xc6, 0x0b, 0xc4, 0x49, 0x18, 0xe7, 0x8f, 0xb5, 0x69, 0x25, 0x64, 0x55, 0x2d, 0x42, 0x3d,
	0x70, 0xa8, 0x88, 0xe4, 0x4a, 0x55, 0x94, 0x43, 0x25, 0x64, 0x73, 0x6a, 0x5d, 0x55, 0x1b, 0xd2,
	0x9c, 0xd7, 0x66, 0x62, 0xa3, 0x00, 0x8b, 0xd8, 0xc5, 0x15, 0x8f, 0xd0, 0x37, 0xe9, 0x83, 0xf4,
	0x5d, 0xfa, 0x1a, 0x15, 0x8b, 0xb1, 0x89, 0x63, 0xb5, 0xb9, 0x0d, 0xdf, 0xfe, 0xbe, 0xe1, 0x9b,
	0xd9, 0x85, 0x13, 0x31, 0x5f, 0x62, 0xca, 0xbc, 0xbc, 0xe0, 0x92, 0x93, 0x9e, 0xac, 0x72, 0x8c,
	0x86, 0x6f, 0x17, 0x9c, 0x2f, 0x12, 0xbc, 0x54, 0xe2, 0xac, 0xbc, 0xbf, 0x94, 0x71, 0x8a, 0x42,
	0xb2, 0x34, 0x6f, 0x38, 0xe7, 0x8f, 0x06, 0xe7, 0x9f, 0xcb, 0x19, 0xde, 0x31, 0x39, 0x5f, 0x52,
	0x14, 0x65, 0x22, 0xc9, 0x15, 0x98, 0x1b, 0xcc, 0xd2, 0x6c, 0xcd, 0xed, 0x8f, 0x86, 0x5e, 0xd3,
	0xc8, 0x6b, 0x1b, 0x79, 0x61, 0x4b, 0xd0, 0x2d, 0x4c, 0x08, 0x1c, 0x3c, 0xc4, 0x59, 0x64, 0xe9,
	0xb6, 0xe6, 0x9a, 0x54, 0xd5, 0xe4, 0x13, 0x98, 0x3f, 0xea, 0xe6, 0x61, 0x95, 0xa3, 0x65, 0xd8,
	0x9a, 0x7b, 0x36, 0xb2, 0x3d, 0x95, 0xce, 0xdb, 0xf9, 0xb1, 0x77, 0xd7, 0x72, 0x74, 0x6b, 0x21,
	0x16, 0x1c, 0xe5, 0xac, 0x4a, 0x38, 0x8b, 0xac, 0x03, 0xd5, 0xb6, 0xfd, 0x74, 0xde, 0x83, 0xb9,
	0x71, 0x90, 0x23, 0x30, 0xfc, 0xc9, 0x64, 0xf0, 0x82, 0x00, 0x1c, 0xde, 0x7e, 0x9b, 0xf8, 0x61,
	0x30, 0xd0, 0xea, 0x7a, 0x12, 0x7c, 0x09, 0xc2, 0x60, 0xa0, 0x3b, 0x3f, 0x75, 0x38, 0xa7, 0x28,
	0x78, 0x59, 0xcc, 0xf1, 0xa6, 0x4c, 0x53, 0x56, 0x54, 0xf5, 0xa4, 0xf7, 0x71, 0x21, 0xe4, 0x0d,
	0x62, 0xf6, 0x9c, 0x49, 0x37, 0x30, 0xf9, 0x08, 0xc7, 0x09, 0x5b, 0x1b, 0xf5, 0xff, 0x1a, 0x37,
	0x2c, 0xb9, 0x06, 0x98, 0x17, 0xc8, 0x24, 0xd6, 0x87, 0x6a, 0x1d, 0xff, 0x76, 0x76, 0x68, 0xe2,
	0xc0, 0x49, 0x84, 0x09, 0x4a, 0x8c, 0x7c, 0x19, 0x64, 0xcd, 0x3a, 0x8e, 0xe9, 0x23, 0x8d, 0xbc,
	0x83, 0xd3, 0x02, 0x13, 0x26, 0x63, 0x9e, 0x89, 0x65, 0x9c, 0x0b, 0xab, 0x67, 0x1b, 0xae, 0x49,
	0x1f, 0x8b, 0xce, 0x2f, 0x0d, 0xfa, 0xc1, 0x0a, 0x33, 0x39, 0xe6, 0x65, 0x26, 0x05, 0x09, 0x61,
	0x90, 0xb2, 0x9c, 0x22, 0x13, 0x3c, 0x0b, 0xb9, 0x12, 0x2d, 0xcd, 0x36, 0xdc, 0xfe, 0xc8, 0x5d,
	0x5f, 0x55, 0x87, 0xf6, 0xa6, 0x3b, 0x68, 0x90, 0xc9, 0xa2, 0xa2, 0x4f, 0x3a, 0x0c, 0xc7, 0xf0,
	0x6a, 0x2f, 0x4a, 0x06, 0x60, 0x3c, 0x60, 0xa5, 0x16, 0x6e, 0xd2, 0xba, 0x24, 0x2f, 0xa1, 0xb7,
	0x62, 0x49, 0x89, 0x6a, 0x97, 0x3d, 0xda, 0x7c, 0x5c, 0xeb, 0x57, 0x9a, 0xf3, 0x5b, 0x83, 0x8b,
	0xf6, 0xda, 0xba, 0x91, 0xbf, 0xc3, 0x59, 0xca, 0xf2, 0x69, 0x9c, 0x85, 0x5c, 0xc9, 0x62, 0x1d,
	0xd8, 0x5b, 0x07, 0xde, 0xe3, 0xa9, 0x83, 0x77, 0x0c, 0x4d, 0xec, 0x9d, 0x2e, 0xc3, 0x5b, 0xb8,
	0xd8, 0x83, 0x75, 0x23, 0x1b, 0x4d, 0x64, 0xb7, 0x1b, 0xb9, 0x3f, 0x22, 0x4f, 0x17, 0xd5, 0x1d,
	0x63, 0x0a, 0xa7, 0xea, 0xad, 0xfa, 0x73, 0x19, 0xaf, 0x62, 0x59, 0x91, 0x37, 0x00, 0x5f, 0xf9,
	0x78, 0xc9, 0xb2, 0x05, 0xfa, 0xcd, 0xb2, 0x0d, 0xda, 0x51, 0xc8, 0x6b, 0x30, 0x9b, 0x3a, 0xf2,
	0xa5, 0xa5, 0xab, 0xe3, 0xad, 0x30, 0x3b, 0x54, 0x4f, 0xe5, 0xc3, 0xdf, 0x00, 0x00, 0x00, 0xff,
	0xff, 0x7e, 0x49, 0x70, 0xa9, 0xf5, 0x03, 0x00, 0x00,
}
