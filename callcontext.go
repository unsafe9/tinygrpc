package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
)

type CallType int

const (
	CallTypeUnary = CallType(iota)
	CallTypeClientStream
	CallTypeServerStream
	CallTypeBidiStream
)

type CallContext struct {
	Ctx        context.Context
	Server     any
	FullMethod string
	CallType   CallType
}

func newUnaryCallContext(ctx context.Context, info *grpc.UnaryServerInfo) *CallContext {
	return &CallContext{
		Ctx:        ctx,
		Server:     info.Server,
		FullMethod: info.FullMethod,
		CallType:   CallTypeUnary,
	}
}

func newStreamCallContext(ctx context.Context, server any, info *grpc.StreamServerInfo) *CallContext {
	var callType CallType
	if info.IsServerStream && info.IsClientStream {
		callType = CallTypeBidiStream
	} else if info.IsServerStream {
		callType = CallTypeServerStream
	} else if info.IsClientStream {
		callType = CallTypeClientStream
	} else {
		panic("invalid stream call type")
	}
	return &CallContext{
		Ctx:        ctx,
		Server:     server,
		FullMethod: info.FullMethod,
		CallType:   callType,
	}
}