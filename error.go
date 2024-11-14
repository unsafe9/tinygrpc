package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
)

type ErrorDeterminer func(c *CallContext, err error) error

func UnaryServerErrorDeterminer(determiner ErrorDeterminer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ret, err := handler(ctx, req)
		return ret, determiner(newUnaryCallContext(ctx, info), err)
	}
}

func StreamServerErrorDeterminer(determiner ErrorDeterminer) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return determiner(newStreamCallContext(ss.Context(), srv, info), handler(srv, ss))
	}
}
