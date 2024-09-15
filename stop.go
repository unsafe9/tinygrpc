package tinygrpc

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"syscall"
)

func GracefulStop(grpcServer *grpc.Server) chan struct{} {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		s := <-sigCh
		logrus.Infof("got signal %v, attempting graceful shutdown", s)
		// cancel()
		grpcServer.GracefulStop()
		// grpcServer.Stop() // leads to error while receiving stream response: rpc error: code = Unavailable desc = transport is closing
		close(done)
	}()

	return done
}
