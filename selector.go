package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
)

type SelectorMatcher func(c *CallContext) bool

func UnaryServerSelector(interceptor grpc.UnaryServerInterceptor, matcher SelectorMatcher) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if matcher(newUnaryCallContext(ctx, info)) {
			return interceptor(ctx, req, info, handler)
		}
		return handler(ctx, req)
	}
}

func StreamServerSelector(interceptor grpc.StreamServerInterceptor, matcher SelectorMatcher) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if matcher(newStreamCallContext(ss.Context(), srv, info)) {
			return interceptor(srv, ss, info, handler)
		}
		return handler(srv, ss)
	}
}
