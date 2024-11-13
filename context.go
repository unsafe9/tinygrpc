package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
)

type ContextInjector func(ctx context.Context) context.Context

func UnaryWithContext(injector ContextInjector) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, retErr error) {
		return handler(injector(ctx), req)
	}
}

func StreamWithContext(injector ContextInjector) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (retErr error) {
		return handler(srv, &serverStreamWithContext{ss, injector(ss.Context())})
	}
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}
