package tinygrpc

import (
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"syscall"
)

func GracefulStop(grpcServer *grpc.Server, hook func(os.Signal)) chan struct{} {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		s := <-sigCh
		if hook != nil {
			hook(s)
		}
		// cancel()
		grpcServer.GracefulStop()
		// grpcServer.Stop() // leads to error while receiving stream response: rpc error: code = Unavailable desc = transport is closing
		close(done)
	}()

	return done
}
