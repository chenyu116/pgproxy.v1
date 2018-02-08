// Code generated by protoc-gen-go. DO NOT EDIT.
// source: api.proto

/*
Package apipb is a generated protocol buffer package.

It is generated from these files:
	api.proto

It has these top-level messages:
	ShutdownRequest
	ShutdownResponse
*/
package apipb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ShutdownRequest struct {
}

func (m *ShutdownRequest) Reset()                    { *m = ShutdownRequest{} }
func (m *ShutdownRequest) String() string            { return proto.CompactTextString(m) }
func (*ShutdownRequest) ProtoMessage()               {}
func (*ShutdownRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type ShutdownResponse struct {
}

func (m *ShutdownResponse) Reset()                    { *m = ShutdownResponse{} }
func (m *ShutdownResponse) String() string            { return proto.CompactTextString(m) }
func (*ShutdownResponse) ProtoMessage()               {}
func (*ShutdownResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func init() {
	proto.RegisterType((*ShutdownRequest)(nil), "pgproxy.server.apipb.ShutdownRequest")
	proto.RegisterType((*ShutdownResponse)(nil), "pgproxy.server.apipb.ShutdownResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Api service

type ApiClient interface {
	Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error)
}

type apiClient struct {
	cc *grpc.ClientConn
}

func NewApiClient(cc *grpc.ClientConn) ApiClient {
	return &apiClient{cc}
}

func (c *apiClient) Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error) {
	out := new(ShutdownResponse)
	err := grpc.Invoke(ctx, "/pgproxy.server.apipb.Api/Shutdown", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Api service

type ApiServer interface {
	Shutdown(context.Context, *ShutdownRequest) (*ShutdownResponse, error)
}

func RegisterApiServer(s *grpc.Server, srv ApiServer) {
	s.RegisterService(&_Api_serviceDesc, srv)
}

func _Api_Shutdown_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ShutdownRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiServer).Shutdown(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pgproxy.server.apipb.Api/Shutdown",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiServer).Shutdown(ctx, req.(*ShutdownRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Api_serviceDesc = grpc.ServiceDesc{
	ServiceName: "pgproxy.server.apipb.Api",
	HandlerType: (*ApiServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Shutdown",
			Handler:    _Api_Shutdown_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api.proto",
}

func init() { proto.RegisterFile("api.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 129 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4c, 0x2c, 0xc8, 0xd4,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x12, 0x29, 0x48, 0x2f, 0x28, 0xca, 0xaf, 0xa8, 0xd4, 0x2b,
	0x4e, 0x2d, 0x2a, 0x4b, 0x2d, 0xd2, 0x4b, 0x2c, 0xc8, 0x2c, 0x48, 0x52, 0x12, 0xe4, 0xe2, 0x0f,
	0xce, 0x28, 0x2d, 0x49, 0xc9, 0x2f, 0xcf, 0x0b, 0x4a, 0x2d, 0x2c, 0x4d, 0x2d, 0x2e, 0x51, 0x12,
	0xe2, 0x12, 0x40, 0x08, 0x15, 0x17, 0xe4, 0xe7, 0x15, 0xa7, 0x1a, 0x25, 0x71, 0x31, 0x3b, 0x16,
	0x64, 0x0a, 0x45, 0x73, 0x71, 0xc0, 0xa4, 0x84, 0x54, 0xf5, 0xb0, 0x19, 0xa8, 0x87, 0x66, 0x9a,
	0x94, 0x1a, 0x21, 0x65, 0x10, 0x1b, 0x94, 0x18, 0x9c, 0xd8, 0xa3, 0x58, 0xc1, 0x72, 0x49, 0x6c,
	0x60, 0x07, 0x1b, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0x33, 0x14, 0x5a, 0x71, 0xbd, 0x00, 0x00,
	0x00,
}