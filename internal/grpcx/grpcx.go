// Package grpcx builds instrumented gRPC servers and clients.
package grpcx

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	handled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_handled_total", Help: "gRPC requests handled."}, []string{"method", "code"})
	latency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "grpc_server_handling_seconds", Help: "gRPC handling latency."}, []string{"method"})
)

func metrics(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := h(ctx, req)
	code := status.Code(err).String()
	handled.WithLabelValues(info.FullMethod, code).Inc()
	latency.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
	slog.InfoContext(ctx, "grpc", "method", info.FullMethod, "code", code, "ms", time.Since(start).Milliseconds())
	return resp, err
}

// NewServer returns a gRPC server with tracing, metrics and request logging.
func NewServer() *grpc.Server {
	return grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(metrics),
	)
}

// Serve listens on addr and blocks serving srv.
func Serve(srv *grpc.Server, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	slog.Info("grpc listening", "addr", addr)
	return srv.Serve(lis)
}

// Dial connects to the Envoy gRPC listener (all inter-service calls go through Envoy).
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}
