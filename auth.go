package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

type AuthProvider interface {
	VerifyAuth(c *CallContext) (context.Context, error)
}

type chainAuthProvider struct {
	providers []AuthProvider
}

func ChainAuthProvider(providers ...AuthProvider) AuthProvider {
	return &chainAuthProvider{providers: providers}
}

func (p *chainAuthProvider) VerifyAuth(c *CallContext) (ctx context.Context, err error) {
	for _, provider := range p.providers {
		ctx, err = provider.VerifyAuth(c)
		if err != nil {
			return
		}
	}
	return
}

type optionalChainAuthProvider struct {
	providers []AuthProvider
}

func OptionalChainAuthProvider(providers ...AuthProvider) AuthProvider {
	return &optionalChainAuthProvider{providers: providers}
}

func (p *optionalChainAuthProvider) VerifyAuth(c *CallContext) (ctx context.Context, err error) {
	// inject context if any provider returns context
	for _, provider := range p.providers {
		ctx, err = provider.VerifyAuth(c)
		if status.Code(err) != codes.Unauthenticated {
			return
		}
	}
	return ctx, nil
}

type AuthHandler func(c *CallContext) (context.Context, error)

func (h AuthHandler) VerifyAuth(c *CallContext) (context.Context, error) {
	return h(c)
}

func ExtractAuthMetadata(ctx context.Context, mdKey, schema string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	sessionIds := md.Get(mdKey)
	if len(sessionIds) != 1 {
		return "", status.Error(codes.Unauthenticated, "invalid auth metadata")
	}
	token := sessionIds[0]

	if schema != "" {
		token = strings.TrimPrefix(token, schema+" ")
	}

	return token, nil
}

func UnaryServerAuthHandler(authProvider AuthProvider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := authProvider.VerifyAuth(newUnaryCallContext(ctx, info))
		if err != nil {
			return nil, err
		}
		if newCtx == nil {
			newCtx = ctx
		}
		return handler(newCtx, req)
	}
}

func StreamServerAuthHandler(authHandler AuthHandler) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		newCtx, err := authHandler(newStreamCallContext(ctx, srv, info))
		if err != nil {
			return err
		}
		if newCtx == nil || newCtx != ctx {
			ss = &serverStreamWithContext{ServerStream: ss, ctx: newCtx}
		}
		return handler(srv, ss)
	}
}
