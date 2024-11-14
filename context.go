package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
)

type ContextInjector func(c *CallContext) context.Context

func UnaryServerContextInjector(injector ContextInjector) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, retErr error) {
		return handler(injector(newUnaryCallContext(ctx, info)), req)
	}
}

func StreamServerContextInjector(injector ContextInjector) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (retErr error) {
		return handler(
			srv,
			&serverStreamWithContext{
				ServerStream: ss,
				ctx:          injector(newStreamCallContext(ss.Context(), srv, info)),
			},
		)
	}
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}
