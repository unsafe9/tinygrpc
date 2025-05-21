package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type CallType int

const (
	CallTypeUnary = CallType(iota)
	CallTypeClientStream
	CallTypeServerStream
	CallTypeBidiStream
)

type CallContext struct {
	ctx        context.Context
	Server     any
	FullMethod string
	CallType   CallType
	Peer       PeerInfo
}

func (c *CallContext) Context() context.Context {
	return c.ctx
}

func newUnaryCallContext(ctx context.Context, info *grpc.UnaryServerInfo) *CallContext {
	return &CallContext{
		ctx:        ctx,
		Server:     info.Server,
		FullMethod: info.FullMethod,
		CallType:   CallTypeUnary,
		Peer:       ParsePeerInfo(ctx),
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
		ctx:        ctx,
		Server:     server,
		FullMethod: info.FullMethod,
		CallType:   callType,
		Peer:       ParsePeerInfo(ctx),
	}
}

type PeerInfo struct {
	Addr      string
	UserAgent string
	Host      string
}

func ParsePeerInfo(ctx context.Context) PeerInfo {
	var peerInfo PeerInfo
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			peerInfo.Addr = vals[0]
		} else if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			peerInfo.Addr = p.Addr.String()
		}
		if vals := md.Get("user-agent"); len(vals) > 0 {
			peerInfo.UserAgent = vals[0]
		}
		if vals := md.Get(":authority"); len(vals) > 0 {
			peerInfo.Host = vals[0]
		}
	}
	return peerInfo
}
