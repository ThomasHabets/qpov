// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v3.21.12
// source: scheduler.proto

package qpov

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	CookieMonster_Login_FullMethodName       = "/qpov.CookieMonster/Login"
	CookieMonster_Logout_FullMethodName      = "/qpov.CookieMonster/Logout"
	CookieMonster_CheckCookie_FullMethodName = "/qpov.CookieMonster/CheckCookie"
)

// CookieMonsterClient is the client API for CookieMonster service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// CookieMonster keeps login state keyed by cookie.
type CookieMonsterClient interface {
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginReply, error)
	Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutReply, error)
	CheckCookie(ctx context.Context, in *CheckCookieRequest, opts ...grpc.CallOption) (*CheckCookieReply, error)
}

type cookieMonsterClient struct {
	cc grpc.ClientConnInterface
}

func NewCookieMonsterClient(cc grpc.ClientConnInterface) CookieMonsterClient {
	return &cookieMonsterClient{cc}
}

func (c *cookieMonsterClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LoginReply)
	err := c.cc.Invoke(ctx, CookieMonster_Login_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cookieMonsterClient) Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LogoutReply)
	err := c.cc.Invoke(ctx, CookieMonster_Logout_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cookieMonsterClient) CheckCookie(ctx context.Context, in *CheckCookieRequest, opts ...grpc.CallOption) (*CheckCookieReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CheckCookieReply)
	err := c.cc.Invoke(ctx, CookieMonster_CheckCookie_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CookieMonsterServer is the server API for CookieMonster service.
// All implementations must embed UnimplementedCookieMonsterServer
// for forward compatibility.
//
// CookieMonster keeps login state keyed by cookie.
type CookieMonsterServer interface {
	Login(context.Context, *LoginRequest) (*LoginReply, error)
	Logout(context.Context, *LogoutRequest) (*LogoutReply, error)
	CheckCookie(context.Context, *CheckCookieRequest) (*CheckCookieReply, error)
	mustEmbedUnimplementedCookieMonsterServer()
}

// UnimplementedCookieMonsterServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedCookieMonsterServer struct{}

func (UnimplementedCookieMonsterServer) Login(context.Context, *LoginRequest) (*LoginReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Login not implemented")
}
func (UnimplementedCookieMonsterServer) Logout(context.Context, *LogoutRequest) (*LogoutReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Logout not implemented")
}
func (UnimplementedCookieMonsterServer) CheckCookie(context.Context, *CheckCookieRequest) (*CheckCookieReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckCookie not implemented")
}
func (UnimplementedCookieMonsterServer) mustEmbedUnimplementedCookieMonsterServer() {}
func (UnimplementedCookieMonsterServer) testEmbeddedByValue()                       {}

// UnsafeCookieMonsterServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CookieMonsterServer will
// result in compilation errors.
type UnsafeCookieMonsterServer interface {
	mustEmbedUnimplementedCookieMonsterServer()
}

func RegisterCookieMonsterServer(s grpc.ServiceRegistrar, srv CookieMonsterServer) {
	// If the following call pancis, it indicates UnimplementedCookieMonsterServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&CookieMonster_ServiceDesc, srv)
}

func _CookieMonster_Login_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CookieMonsterServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CookieMonster_Login_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CookieMonsterServer).Login(ctx, req.(*LoginRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CookieMonster_Logout_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogoutRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CookieMonsterServer).Logout(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CookieMonster_Logout_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CookieMonsterServer).Logout(ctx, req.(*LogoutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CookieMonster_CheckCookie_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckCookieRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CookieMonsterServer).CheckCookie(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CookieMonster_CheckCookie_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CookieMonsterServer).CheckCookie(ctx, req.(*CheckCookieRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// CookieMonster_ServiceDesc is the grpc.ServiceDesc for CookieMonster service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var CookieMonster_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "qpov.CookieMonster",
	HandlerType: (*CookieMonsterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Login",
			Handler:    _CookieMonster_Login_Handler,
		},
		{
			MethodName: "Logout",
			Handler:    _CookieMonster_Logout_Handler,
		},
		{
			MethodName: "CheckCookie",
			Handler:    _CookieMonster_CheckCookie_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "scheduler.proto",
}

const (
	Scheduler_Get_FullMethodName         = "/qpov.Scheduler/Get"
	Scheduler_Renew_FullMethodName       = "/qpov.Scheduler/Renew"
	Scheduler_Done_FullMethodName        = "/qpov.Scheduler/Done"
	Scheduler_Failed_FullMethodName      = "/qpov.Scheduler/Failed"
	Scheduler_Add_FullMethodName         = "/qpov.Scheduler/Add"
	Scheduler_Lease_FullMethodName       = "/qpov.Scheduler/Lease"
	Scheduler_Leases_FullMethodName      = "/qpov.Scheduler/Leases"
	Scheduler_Order_FullMethodName       = "/qpov.Scheduler/Order"
	Scheduler_Orders_FullMethodName      = "/qpov.Scheduler/Orders"
	Scheduler_Stats_FullMethodName       = "/qpov.Scheduler/Stats"
	Scheduler_Result_FullMethodName      = "/qpov.Scheduler/Result"
	Scheduler_Certificate_FullMethodName = "/qpov.Scheduler/Certificate"
)

// SchedulerClient is the client API for Scheduler service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SchedulerClient interface {
	// Render client API.
	Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetReply, error)
	Renew(ctx context.Context, in *RenewRequest, opts ...grpc.CallOption) (*RenewReply, error)
	Done(ctx context.Context, in *DoneRequest, opts ...grpc.CallOption) (*DoneReply, error)
	Failed(ctx context.Context, in *FailedRequest, opts ...grpc.CallOption) (*FailedReply, error)
	// Order handling API. Restricted.
	Add(ctx context.Context, in *AddRequest, opts ...grpc.CallOption) (*AddReply, error)
	// Stats API. Restricted.
	Lease(ctx context.Context, in *LeaseRequest, opts ...grpc.CallOption) (*LeaseReply, error)
	Leases(ctx context.Context, in *LeasesRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LeasesReply], error)
	Order(ctx context.Context, in *OrderRequest, opts ...grpc.CallOption) (*OrderReply, error)
	Orders(ctx context.Context, in *OrdersRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[OrdersReply], error)
	// Stats API, unrestricted.
	Stats(ctx context.Context, in *StatsRequest, opts ...grpc.CallOption) (*StatsReply, error)
	// WebUI magic.
	// rpc UserStats (UserStatsRequest) returns (UserStatsReply) {}
	Result(ctx context.Context, in *ResultRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ResultReply], error)
	Certificate(ctx context.Context, in *CertificateRequest, opts ...grpc.CallOption) (*CertificateReply, error)
}

type schedulerClient struct {
	cc grpc.ClientConnInterface
}

func NewSchedulerClient(cc grpc.ClientConnInterface) SchedulerClient {
	return &schedulerClient{cc}
}

func (c *schedulerClient) Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetReply)
	err := c.cc.Invoke(ctx, Scheduler_Get_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Renew(ctx context.Context, in *RenewRequest, opts ...grpc.CallOption) (*RenewReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RenewReply)
	err := c.cc.Invoke(ctx, Scheduler_Renew_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Done(ctx context.Context, in *DoneRequest, opts ...grpc.CallOption) (*DoneReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DoneReply)
	err := c.cc.Invoke(ctx, Scheduler_Done_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Failed(ctx context.Context, in *FailedRequest, opts ...grpc.CallOption) (*FailedReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(FailedReply)
	err := c.cc.Invoke(ctx, Scheduler_Failed_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Add(ctx context.Context, in *AddRequest, opts ...grpc.CallOption) (*AddReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddReply)
	err := c.cc.Invoke(ctx, Scheduler_Add_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Lease(ctx context.Context, in *LeaseRequest, opts ...grpc.CallOption) (*LeaseReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LeaseReply)
	err := c.cc.Invoke(ctx, Scheduler_Lease_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Leases(ctx context.Context, in *LeasesRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LeasesReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Scheduler_ServiceDesc.Streams[0], Scheduler_Leases_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[LeasesRequest, LeasesReply]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Scheduler_LeasesClient = grpc.ServerStreamingClient[LeasesReply]

func (c *schedulerClient) Order(ctx context.Context, in *OrderRequest, opts ...grpc.CallOption) (*OrderReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(OrderReply)
	err := c.cc.Invoke(ctx, Scheduler_Order_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Orders(ctx context.Context, in *OrdersRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[OrdersReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Scheduler_ServiceDesc.Streams[1], Scheduler_Orders_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[OrdersRequest, OrdersReply]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Scheduler_OrdersClient = grpc.ServerStreamingClient[OrdersReply]

func (c *schedulerClient) Stats(ctx context.Context, in *StatsRequest, opts ...grpc.CallOption) (*StatsReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(StatsReply)
	err := c.cc.Invoke(ctx, Scheduler_Stats_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) Result(ctx context.Context, in *ResultRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ResultReply], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Scheduler_ServiceDesc.Streams[2], Scheduler_Result_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ResultRequest, ResultReply]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Scheduler_ResultClient = grpc.ServerStreamingClient[ResultReply]

func (c *schedulerClient) Certificate(ctx context.Context, in *CertificateRequest, opts ...grpc.CallOption) (*CertificateReply, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CertificateReply)
	err := c.cc.Invoke(ctx, Scheduler_Certificate_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SchedulerServer is the server API for Scheduler service.
// All implementations must embed UnimplementedSchedulerServer
// for forward compatibility.
type SchedulerServer interface {
	// Render client API.
	Get(context.Context, *GetRequest) (*GetReply, error)
	Renew(context.Context, *RenewRequest) (*RenewReply, error)
	Done(context.Context, *DoneRequest) (*DoneReply, error)
	Failed(context.Context, *FailedRequest) (*FailedReply, error)
	// Order handling API. Restricted.
	Add(context.Context, *AddRequest) (*AddReply, error)
	// Stats API. Restricted.
	Lease(context.Context, *LeaseRequest) (*LeaseReply, error)
	Leases(*LeasesRequest, grpc.ServerStreamingServer[LeasesReply]) error
	Order(context.Context, *OrderRequest) (*OrderReply, error)
	Orders(*OrdersRequest, grpc.ServerStreamingServer[OrdersReply]) error
	// Stats API, unrestricted.
	Stats(context.Context, *StatsRequest) (*StatsReply, error)
	// WebUI magic.
	// rpc UserStats (UserStatsRequest) returns (UserStatsReply) {}
	Result(*ResultRequest, grpc.ServerStreamingServer[ResultReply]) error
	Certificate(context.Context, *CertificateRequest) (*CertificateReply, error)
	mustEmbedUnimplementedSchedulerServer()
}

// UnimplementedSchedulerServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSchedulerServer struct{}

func (UnimplementedSchedulerServer) Get(context.Context, *GetRequest) (*GetReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedSchedulerServer) Renew(context.Context, *RenewRequest) (*RenewReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Renew not implemented")
}
func (UnimplementedSchedulerServer) Done(context.Context, *DoneRequest) (*DoneReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Done not implemented")
}
func (UnimplementedSchedulerServer) Failed(context.Context, *FailedRequest) (*FailedReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Failed not implemented")
}
func (UnimplementedSchedulerServer) Add(context.Context, *AddRequest) (*AddReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Add not implemented")
}
func (UnimplementedSchedulerServer) Lease(context.Context, *LeaseRequest) (*LeaseReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Lease not implemented")
}
func (UnimplementedSchedulerServer) Leases(*LeasesRequest, grpc.ServerStreamingServer[LeasesReply]) error {
	return status.Errorf(codes.Unimplemented, "method Leases not implemented")
}
func (UnimplementedSchedulerServer) Order(context.Context, *OrderRequest) (*OrderReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Order not implemented")
}
func (UnimplementedSchedulerServer) Orders(*OrdersRequest, grpc.ServerStreamingServer[OrdersReply]) error {
	return status.Errorf(codes.Unimplemented, "method Orders not implemented")
}
func (UnimplementedSchedulerServer) Stats(context.Context, *StatsRequest) (*StatsReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stats not implemented")
}
func (UnimplementedSchedulerServer) Result(*ResultRequest, grpc.ServerStreamingServer[ResultReply]) error {
	return status.Errorf(codes.Unimplemented, "method Result not implemented")
}
func (UnimplementedSchedulerServer) Certificate(context.Context, *CertificateRequest) (*CertificateReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Certificate not implemented")
}
func (UnimplementedSchedulerServer) mustEmbedUnimplementedSchedulerServer() {}
func (UnimplementedSchedulerServer) testEmbeddedByValue()                   {}

// UnsafeSchedulerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SchedulerServer will
// result in compilation errors.
type UnsafeSchedulerServer interface {
	mustEmbedUnimplementedSchedulerServer()
}

func RegisterSchedulerServer(s grpc.ServiceRegistrar, srv SchedulerServer) {
	// If the following call pancis, it indicates UnimplementedSchedulerServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Scheduler_ServiceDesc, srv)
}

func _Scheduler_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Get_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Get(ctx, req.(*GetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Renew_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RenewRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Renew(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Renew_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Renew(ctx, req.(*RenewRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Done_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DoneRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Done(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Done_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Done(ctx, req.(*DoneRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Failed_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FailedRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Failed(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Failed_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Failed(ctx, req.(*FailedRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Add_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Add(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Add_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Add(ctx, req.(*AddRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Lease_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LeaseRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Lease(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Lease_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Lease(ctx, req.(*LeaseRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Leases_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(LeasesRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SchedulerServer).Leases(m, &grpc.GenericServerStream[LeasesRequest, LeasesReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Scheduler_LeasesServer = grpc.ServerStreamingServer[LeasesReply]

func _Scheduler_Order_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OrderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Order(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Order_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Order(ctx, req.(*OrderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Orders_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(OrdersRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SchedulerServer).Orders(m, &grpc.GenericServerStream[OrdersRequest, OrdersReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Scheduler_OrdersServer = grpc.ServerStreamingServer[OrdersReply]

func _Scheduler_Stats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Stats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Stats_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Stats(ctx, req.(*StatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_Result_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ResultRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SchedulerServer).Result(m, &grpc.GenericServerStream[ResultRequest, ResultReply]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Scheduler_ResultServer = grpc.ServerStreamingServer[ResultReply]

func _Scheduler_Certificate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CertificateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Certificate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scheduler_Certificate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Certificate(ctx, req.(*CertificateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Scheduler_ServiceDesc is the grpc.ServiceDesc for Scheduler service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Scheduler_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "qpov.Scheduler",
	HandlerType: (*SchedulerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Get",
			Handler:    _Scheduler_Get_Handler,
		},
		{
			MethodName: "Renew",
			Handler:    _Scheduler_Renew_Handler,
		},
		{
			MethodName: "Done",
			Handler:    _Scheduler_Done_Handler,
		},
		{
			MethodName: "Failed",
			Handler:    _Scheduler_Failed_Handler,
		},
		{
			MethodName: "Add",
			Handler:    _Scheduler_Add_Handler,
		},
		{
			MethodName: "Lease",
			Handler:    _Scheduler_Lease_Handler,
		},
		{
			MethodName: "Order",
			Handler:    _Scheduler_Order_Handler,
		},
		{
			MethodName: "Stats",
			Handler:    _Scheduler_Stats_Handler,
		},
		{
			MethodName: "Certificate",
			Handler:    _Scheduler_Certificate_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Leases",
			Handler:       _Scheduler_Leases_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Orders",
			Handler:       _Scheduler_Orders_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Result",
			Handler:       _Scheduler_Result_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "scheduler.proto",
}
