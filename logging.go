package tinygrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"time"
)

type UnaryLogger interface {
	LogUnaryRequest(c *CallContext, req proto.Message)
	LogUnaryResponse(c *CallContext, duration time.Duration, req, res proto.Message, err error)
}

type StreamLogger interface {
	LogStreamConnect(c *CallContext)
	LogStreamDisconnect(c *CallContext, duration time.Duration, err error)
	LogStreamSendMsg(c *CallContext, message proto.Message, err error)
	LogStreamRecvMsg(c *CallContext, message proto.Message, err error)
}

func UnaryServerLogger(logger UnaryLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		callCtx := newUnaryCallContext(ctx, info)
		logger.LogUnaryRequest(callCtx, req.(proto.Message))

		start := time.Now()
		res, err := handler(ctx, req)
		duration := time.Since(start)

		logger.LogUnaryResponse(callCtx, duration, req.(proto.Message), res.(proto.Message), err)
		return res, err
	}
}

func StreamServerLogger(logger StreamLogger, logPayload bool) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		callCtx := newStreamCallContext(ctx, srv, info)
		logger.LogStreamConnect(callCtx)

		if logPayload {
			ss = &loggingServerStream{
				ServerStream: ss,
				callCtx:      callCtx,
				logger:       logger,
			}
		}

		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		logger.LogStreamDisconnect(callCtx, duration, err)
		return err
	}
}

type loggingServerStream struct {
	grpc.ServerStream
	callCtx *CallContext
	logger  StreamLogger
}

func (l *loggingServerStream) SendMsg(m any) error {
	err := l.ServerStream.SendMsg(m)
	l.logger.LogStreamSendMsg(l.callCtx, m.(proto.Message), err)
	return err
}

func (l *loggingServerStream) RecvMsg(m any) error {
	err := l.ServerStream.RecvMsg(m)
	l.logger.LogStreamRecvMsg(l.callCtx, m.(proto.Message), err)
	return err
}
