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

var apiLogger *logrus.Logger

func SetLogger(logger *logrus.Logger) {
	apiLogger = logger
}

func LogrusKeySortingFunc(keys []string) {
	order := []string{
		logrus.FieldKeyTime,
		logrus.FieldKeyLevel,
		"method",
		"code",
		"duration",
		"req",
		"res",
		"peer_ip",
		"user_agent",
		"host",
		logrus.ErrorKey,
	}

	sort.SliceStable(keys, func(i, j int) bool {
		io := slices.Index(order, keys[i])
		jo := slices.Index(order, keys[j])
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

func metadataVars(ctx context.Context) (peerAddr, userAgent, host string) {
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

func UnaryLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	res, err := handler(ctx, req)
	duration := time.Since(start)

	var (
		reqStr string
		resStr string
	)
	if req != nil {
		reqMarshal, _ := protojson.Marshal(req.(proto.Message))
		reqStr = string(reqMarshal)
	}
	if res != nil {
		resMarshal, _ := protojson.Marshal(res.(proto.Message))
		resStr = string(resMarshal)
	}
	peerAddr, userAgent, host := metadataVars(ctx)

	entry := apiLogger.WithFields(logrus.Fields{
		"method":     info.FullMethod,
		"status":     status.Code(err).String(),
		"duration":   duration.String(),
		"req":        reqStr,
		"res":        resStr,
		"peer":       peerAddr,
		"user_agent": userAgent,
		"host":       host,
	})
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Info()

	return res, err
}

func StreamLoggingInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		ss = &loggingServerStream{
			ServerStream: ss,
			method:       info.FullMethod,
		}
	}
	err := handler(srv, ss)
	duration := time.Since(start)

	peerAddr, userAgent, host := metadataVars(ss.Context())

	entry := apiLogger.WithFields(logrus.Fields{
		"method":     info.FullMethod,
		"status":     status.Code(err).String(),
		"duration":   duration.String(),
		"peer":       peerAddr,
		"user_agent": userAgent,
		"host":       host,
	})
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Info()

	return err
}

type loggingServerStream struct {
	grpc.ServerStream
	method string
}

func (l *loggingServerStream) SendMsg(m interface{}) error {
	start := time.Now()
	err := l.ServerStream.SendMsg(m)
	duration := time.Since(start)

	var resStr string
	if m != nil {
		resMarshal, _ := protojson.Marshal(m.(proto.Message))
		resStr = string(resMarshal)
	}

	entry := apiLogger.WithFields(logrus.Fields{
		"method":   l.method,
		"event":    "send",
		"duration": duration.String(),
		"res":      resStr,
	})
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Debug()

	return err
}

func (l *loggingServerStream) RecvMsg(m interface{}) error {
	start := time.Now()
	err := l.ServerStream.RecvMsg(m)
	duration := time.Since(start)

	var reqStr string
	if m != nil {
		reqMarshal, _ := protojson.Marshal(m.(proto.Message))
		reqStr = string(reqMarshal)
	}

	entry := apiLogger.WithFields(logrus.Fields{
		"method":   l.method,
		"event":    "receive",
		"duration": duration.String(),
		"req":      reqStr,
	})
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Debug()

	return err
}
