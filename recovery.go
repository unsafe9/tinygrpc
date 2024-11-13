package tinygrpc

import (
	"context"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"runtime/debug"
)

func UnaryRecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithFields(logrus.Fields{
				"method": info.FullMethod,
				"panic":  r,
				"stack":  string(debug.Stack()),
			}).Error()

			retErr = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}

func StreamRecoveryInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithFields(logrus.Fields{
				"method": info.FullMethod,
				"panic":  r,
				"stack":  string(debug.Stack()),
			}).Error()

			retErr = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(srv, ss)
}
