package tinygrpc

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"runtime"
)

type RecoveryHandler func(c *CallContext, p *PanicWrapper) error

type PanicWrapper struct {
	Panic any
	Stack []byte
}

func (p PanicWrapper) Error() string {
	return fmt.Sprintf("panic caught: %v\n%s", p.Panic, p.Stack)
}

func wrapPanic(p any) *PanicWrapper {
	stack := make([]byte, 64<<10)
	stack = stack[:runtime.Stack(stack, false)]
	return &PanicWrapper{
		Panic: p,
		Stack: stack,
	}
}

func UnaryServerRecoveryHandler(recoveryHandler RecoveryHandler) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, retErr error) {
		defer func() {
			if r := recover(); r != nil {
				p := wrapPanic(r)
				if recoveryHandler != nil {
					retErr = recoveryHandler(newUnaryCallContext(ctx, info), p)
				} else {
					retErr = p
				}
			}
		}()
		return handler(ctx, req)
	}
}

func StreamServerRecoveryHandler(recoveryHandler RecoveryHandler) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				p := wrapPanic(r)
				if recoveryHandler != nil {
					retErr = recoveryHandler(newStreamCallContext(ss.Context(), srv, info), p)
				} else {
					retErr = p
				}
			}
		}()
		return handler(srv, ss)
	}
}
