package tinygrpc

import (
	"context"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"slices"
	"sort"
	"time"
)

type logCtxKeyType struct{}

var logCtxKey = logCtxKeyType{}

type logContextValue struct {
	additionalFields logrus.Fields
	skipKeys         map[string]struct{}
}

func WithLogFields(ctx context.Context, fields logrus.Fields) context.Context {
	var val logContextValue
	if ctxVal := ctx.Value(logCtxKey); ctxVal != nil {
		val = ctxVal.(logContextValue)
	}
	if val.additionalFields == nil {
		val.additionalFields = make(logrus.Fields)
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

func getLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {
	if ctxVal := ctx.Value(logCtxKey); ctxVal != nil {
		val := ctxVal.(logContextValue)
		for k, v := range val.additionalFields {
			fields[k] = v
		}
		for k := range val.skipKeys {
			delete(fields, k)
		}
	}
	return fields
}

var pj = protojson.MarshalOptions{
	EmitDefaultValues: true,
}

type LogLevelDeterminer func(c *CallContext, err error) logrus.Level

func defaultLogLevelDeterminer(c *CallContext, err error) logrus.Level {
	if err != nil {
		return logrus.ErrorLevel
	}
	return logrus.InfoLevel
}

var LogFieldsOrder = []string{
	logrus.FieldKeyTime,
	logrus.FieldKeyLevel,
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
	logrus.ErrorKey,
}

func SortLogFields(keys []string) {
	sort.SliceStable(keys, func(i, j int) bool {
		io := slices.Index(LogFieldsOrder, keys[i])
		jo := slices.Index(LogFieldsOrder, keys[j])
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

func UnaryServerLogger(logger *logrus.Logger, determiner LogLevelDeterminer) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	if determiner == nil {
		determiner = defaultLogLevelDeterminer
	}
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

		entry := logger.WithFields(
			getLogFields(
				ctx,
				logrus.Fields{
					"method":     info.FullMethod,
					"code":       status.Code(err).String(),
					"duration":   duration.String(),
					"req":        reqStr,
					"res":        resStr,
					"peer":       peerAddr,
					"user_agent": userAgent,
					"host":       host,
				},
			),
		)
		if err != nil {
			entry = entry.WithError(err)
		}
		entry.Log(determiner(newUnaryCallContext(ctx, info), err))

		return res, err
	}
}

func StreamServerLogger(logger *logrus.Logger, determiner LogLevelDeterminer, payloadLevel logrus.Level) grpc.StreamServerInterceptor {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	if determiner == nil {
		determiner = defaultLogLevelDeterminer
	}
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		if logrus.IsLevelEnabled(payloadLevel) {
			ss = &loggingServerStream{
				ServerStream: ss,
				method:       info.FullMethod,
				logger:       logger,
			}
		}
		err := handler(srv, ss)
		duration := time.Since(start)

		ctx := ss.Context()
		peerAddr, userAgent, host := parseAdditionalPeerInfo(ctx)

		entry := logger.WithFields(
			getLogFields(
				ctx,
				logrus.Fields{
					"method":     info.FullMethod,
					"code":       status.Code(err).String(),
					"duration":   duration.String(),
					"peer":       peerAddr,
					"user_agent": userAgent,
					"host":       host,
				},
			),
		)
		if err != nil {
			entry = entry.WithError(err)
		}
		entry.Log(determiner(newStreamCallContext(ctx, srv, info), err))

		return err
	}
}

type loggingServerStream struct {
	grpc.ServerStream
	method string
	logger *logrus.Logger
	level  logrus.Level
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

	entry := l.logger.WithFields(logrus.Fields{
		"method":   l.method,
		"event":    "send",
		"duration": duration.String(),
		"res":      resStr,
	})
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Log(l.level)

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

	entry := l.logger.WithFields(logrus.Fields{
		"method":   l.method,
		"event":    "recv",
		"duration": duration.String(),
		"req":      reqStr,
	})
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Log(l.level)

	return err
}
