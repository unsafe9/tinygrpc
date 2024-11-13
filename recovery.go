package tinygrpc

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"runtime/debug"
)

type PanicWrapper struct {
	Panic any
	Stack string
}

func (p PanicWrapper) Error() string {
	return fmt.Sprintf("%v: %s", p.Panic, p.Stack)
}

func UnaryRecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = &PanicWrapper{
				Panic: r,
				Stack: string(debug.Stack()),
			}
		}
	}()
	return handler(ctx, req)
}

func StreamRecoveryInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = &PanicWrapper{
				Panic: r,
				Stack: string(debug.Stack()),
			}
		}
	}()
	return handler(srv, ss)
}
