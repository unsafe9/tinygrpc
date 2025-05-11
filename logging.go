package tinygrpc

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"slices"
	"sort"
	"sync/atomic"
	"time"
)

type LogFields map[string]any

var logLimitBytes atomic.Int32

func SetLogLimitBytes(limit int32) {
	logLimitBytes.Store(limit)
}

type logCtxKeyType struct{}

var logCtxKey = logCtxKeyType{}

type logContextValue struct {
	additionalFields LogFields
	skipKeys         map[string]struct{}
}

func WithLogFields(ctx context.Context, fields LogFields) context.Context {
	var val logContextValue
	if ctxVal := ctx.Value(logCtxKey); ctxVal != nil {
		val = ctxVal.(logContextValue)
	}
	if val.additionalFields == nil {
		val.additionalFields = make(LogFields)
	}
	for k, v := range fields {
		val.additionalFields[k] = v
	}
	return context.WithValue(ctx, logCtxKey, val)
}

func WithSkipLogKeys(ctx context.Context, keys ...string) context.Context {
	var val logContextValue
	if ctxVal := ctx.Value(logCtxKey); ctxVal != nil {
		val = ctxVal.(logContextValue)
	}
	if val.skipKeys == nil {
		val.skipKeys = make(map[string]struct{})
	}
	for _, k := range keys {
		val.skipKeys[k] = struct{}{}
	}
	return context.WithValue(ctx, logCtxKey, val)
}

func getLogFields(ctx context.Context, fields LogFields) LogFields {
	if ctxVal := ctx.Value(logCtxKey); ctxVal != nil {
		val := ctxVal.(logContextValue)
		for k, v := range val.additionalFields {
			fields[k] = v
		}
		for k := range val.skipKeys {
			delete(fields, k)
		}
	}

	limitBytes := int(logLimitBytes.Load())
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			if len(val) > limitBytes {
				fields[k] = fmt.Sprintf("[truncated %d/%d] %s...", limitBytes, len(val), val[:limitBytes])
			}
		case []byte:
			if len(val) > limitBytes {
				fields[k] = val[:limitBytes]
			}
		}
	}
	return fields
}

var pj = protojson.MarshalOptions{
	EmitDefaultValues: true,
}

type LogFunc func(c *CallContext, fields LogFields, err error)

var LogFieldsInOrder = []string{
	"time",
	"level",
	"method",
	"code",
	"duration",
	"event",
	"req",
	"res",
	"user", // reserved
	"auth", // reserved
	"peer",
	"user_agent",
	"host",
	"error",
}

func SortLogFields(keys []string) {
	sort.SliceStable(keys, func(i, j int) bool {
		io := slices.Index(LogFieldsInOrder, keys[i])
		jo := slices.Index(LogFieldsInOrder, keys[j])
		if io == -1 && jo == -1 {
			return true
		} else if io == -1 {
			return false
		} else if jo == -1 {
			return true
		}
		return io < jo
	})
}

func parseAdditionalPeerInfo(ctx context.Context) (peerAddr, userAgent, host string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			peerAddr = vals[0]
		} else if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			peerAddr = p.Addr.String()
		}
		if vals := md.Get("user-agent"); len(vals) > 0 {
			userAgent = vals[0]
		}
		if vals := md.Get(":authority"); len(vals) > 0 {
			host = vals[0]
		}
	}
	return
}

func UnaryServerLogger(logFunc LogFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		res, err := handler(ctx, req)
		duration := time.Since(start)

		var (
			reqStr string
			resStr string
		)
		if req != nil {
			reqMarshal, _ := pj.Marshal(req.(proto.Message))
			reqStr = string(reqMarshal)
		}
		if res != nil {
			resMarshal, _ := pj.Marshal(res.(proto.Message))
			resStr = string(resMarshal)
		}
		peerAddr, userAgent, host := parseAdditionalPeerInfo(ctx)

		logFields := getLogFields(
			ctx,
			LogFields{
				"method":     info.FullMethod,
				"code":       status.Code(err).String(),
				"duration":   duration.String(),
				"req":        reqStr,
				"res":        resStr,
				"peer":       peerAddr,
				"user_agent": userAgent,
				"host":       host,
			},
		)
		if err != nil {
			logFields["error"] = err.Error()
		}
		logFunc(newUnaryCallContext(ctx, info), logFields, err)

		return res, err
	}
}

func StreamServerLogger(logFunc LogFunc, logPayload bool) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		ctx := ss.Context()
		callCtx := newStreamCallContext(ctx, srv, info)

		if logPayload {
			ss = &loggingServerStream{
				ServerStream: ss,
				callCtx:      callCtx,
				method:       info.FullMethod,
				logFunc:      logFunc,
			}
		}

		peerAddr, userAgent, host := parseAdditionalPeerInfo(ctx)

		logFields := getLogFields(
			ctx,
			LogFields{
				"method":     info.FullMethod,
				"code":       status.Code(err).String(),
				"duration":   duration.String(),
				"peer":       peerAddr,
				"user_agent": userAgent,
				"host":       host,
			},
		)
		if err != nil {
			logFields["error"] = err.Error()
		}
		logFunc(callCtx, logFields, err)

		return err
	}
}

type loggingServerStream struct {
	grpc.ServerStream
	callCtx *CallContext
	method  string
	logFunc LogFunc
}

func (l *loggingServerStream) SendMsg(m any) error {
	start := time.Now()
	err := l.ServerStream.SendMsg(m)
	duration := time.Since(start)

	var resStr string
	if m != nil {
		resMarshal, _ := pj.Marshal(m.(proto.Message))
		resStr = string(resMarshal)
	}

	logFields := LogFields{
		"method":   l.method,
		"event":    "send",
		"duration": duration.String(),
		"res":      resStr,
	}
	if err != nil {
		logFields["error"] = err.Error()
	}
	l.logFunc(l.callCtx, logFields, err)

	return err
}

func (l *loggingServerStream) RecvMsg(m any) error {
	start := time.Now()
	err := l.ServerStream.RecvMsg(m)
	duration := time.Since(start)

	var reqStr string
	if m != nil {
		reqMarshal, _ := pj.Marshal(m.(proto.Message))
		reqStr = string(reqMarshal)
	}

	logFields := LogFields{
		"method":   l.method,
		"event":    "recv",
		"duration": duration.String(),
		"req":      reqStr,
	}
	if err != nil {
		logFields["error"] = err.Error()
	}
	l.logFunc(l.callCtx, logFields, err)

	return err
}
