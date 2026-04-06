package encoding

import (
	"context"

	"google.golang.org/grpc"
)

func ServerDesc(name, tun, tunMulti string) grpc.ServiceDesc {
	return grpc.ServiceDesc{
		ServiceName: name,
		HandlerType: (*GRPCServiceServer)(nil),
		Methods:     []grpc.MethodDesc{},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    tun,
				Handler:       _GRPCService_Tun_Handler,
				ServerStreams: true,
				ClientStreams: true,
			},
			{
				StreamName:    tunMulti,
				Handler:       _GRPCService_TunMulti_Handler,
				ServerStreams: true,
				ClientStreams: true,
			},
		},
		Metadata: "grpc.proto",
	}
}

func (c *gRPCServiceClient) TunCustomName(ctx context.Context, name, tun string, opts ...grpc.CallOption) (GRPCService_TunClient, error) {
	stream, err := c.cc.NewStream(ctx, &ServerDesc(name, tun, "").Streams[0], "/"+name+"/"+tun, opts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[Hunk, Hunk]{ClientStream: stream}
	return x, nil
}

func (c *gRPCServiceClient) TunMultiCustomName(ctx context.Context, name, tunMulti string, opts ...grpc.CallOption) (GRPCService_TunMultiClient, error) {
	stream, err := c.cc.NewStream(ctx, &ServerDesc(name, "", tunMulti).Streams[1], "/"+name+"/"+tunMulti, opts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[MultiHunk, MultiHunk]{ClientStream: stream}
	return x, nil
}

type GRPCServiceClientX interface {
	TunCustomName(ctx context.Context, name, tun string, opts ...grpc.CallOption) (GRPCService_TunClient, error)
	TunMultiCustomName(ctx context.Context, name, tunMulti string, opts ...grpc.CallOption) (GRPCService_TunMultiClient, error)
	Tun(ctx context.Context, opts ...grpc.CallOption) (GRPCService_TunClient, error)
	TunMulti(ctx context.Context, opts ...grpc.CallOption) (GRPCService_TunMultiClient, error)
}

func RegisterGRPCServiceServerX(s *grpc.Server, srv GRPCServiceServer, name, tun, tunMulti string) {
	desc := ServerDesc(name, tun, tunMulti)
	s.RegisterService(&desc, srv)
}
