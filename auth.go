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

type MultiAuthProvider struct {
	// Providers is a list of auth providers to be chained
	Providers []AuthProvider

	// AllowUnauthenticated allows the process to pass without authentication
	//  if all providers fail.
	AllowUnauthenticated bool

	// EvaluateAll ensures that all providers in the chain are evaluated,
	//  injecting combined contexts from all successful providers, even if one succeeds early.
	EvaluateAll bool

	// RequireAll requires all providers to succeed, otherwise it will return an error.
	RequireAll bool
}

func (p *MultiAuthProvider) VerifyAuth(c *CallContext) (context.Context, error) {
	var (
		ctx           context.Context
		err           error
		authenticated bool
	)
	for _, provider := range p.Providers {
		ctx, err = provider.VerifyAuth(c)
		if err != nil {
			if p.RequireAll || status.Code(err) != codes.Unauthenticated {
				return ctx, err
			}
		} else {
			authenticated = true
			if !p.RequireAll && !p.EvaluateAll {
				break
			}
			if ctx != nil {
				c.ctx = ctx // inject context for next provider
			}
		}
	}
	if !authenticated && !p.AllowUnauthenticated {
		return ctx, status.Error(codes.Unauthenticated, "all auth providers failed")
	}
	return ctx, nil
}

type AuthProviderFunc func(c *CallContext) (context.Context, error)

func (f AuthProviderFunc) VerifyAuth(c *CallContext) (context.Context, error) {
	return f(c)
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

func StreamServerAuthHandler(authProvider AuthProvider) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		newCtx, err := authProvider.VerifyAuth(newStreamCallContext(ctx, srv, info))
		if err != nil {
			return err
		}
		if newCtx == nil || newCtx != ctx {
			ss = &serverStreamWithContext{ServerStream: ss, ctx: newCtx}
		}
		return handler(srv, ss)
	}
}
