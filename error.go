package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
)

type (
	UnaryErrorDeterminer  func(ctx context.Context, info *grpc.UnaryServerInfo, err error) error
	StreamErrorDeterminer func(ctx context.Context, info *grpc.StreamServerInfo, err error) error
)

func NewUnaryErrorInterceptor(determiner UnaryErrorDeterminer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ret, err := handler(ctx, req)
		if err != nil {
			err = determiner(ctx, info, err)
		}
		return ret, err
	}
}

func NewStreamErrorInterceptor(determiner StreamErrorDeterminer) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		if err != nil {
			err = determiner(ss.Context(), info, err)
		}
		return err
	}
}
