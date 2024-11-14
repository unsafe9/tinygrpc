package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

type AuthHandler func(c *CallContext, token string) (context.Context, error)

func extractAuthMetadata(ctx context.Context, mdKey, schema string) (string, error) {
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

func UnaryServerAuthHandler(mdKey, schema string, authHandler AuthHandler) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		token, err := extractAuthMetadata(ctx, mdKey, schema)
		if err != nil {
			return nil, err
		}
		newCtx, err := authHandler(newUnaryCallContext(ctx, info), token)
		if err != nil {
			return nil, err
		}
		if newCtx == nil {
			newCtx = ctx
		}
		return handler(newCtx, req)
	}
}

func StreamServerAuthHandler(mdKey, schema string, authHandler AuthHandler) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		token, err := extractAuthMetadata(ss.Context(), mdKey, schema)
		if err != nil {
			return err
		}
		ctx, err := authHandler(newStreamCallContext(ss.Context(), srv, info), token)
		if err != nil {
			return err
		}
		if ctx != nil || ctx != ss.Context() {
			ss = &serverStreamWithContext{ServerStream: ss, ctx: ctx}
		}
		return handler(srv, ss)
	}
}
