// Package obs wires structured logging, OpenTelemetry tracing and Prometheus metrics.
package obs

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init sets the default slog logger (JSON on stdout) and, when
// OTEL_EXPORTER_OTLP_ENDPOINT is set, an OTLP/gRPC tracer provider.
func Init(ctx context.Context, service string) (shutdown func(context.Context) error) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", service))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	ep := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if ep == "" {
		return func(context.Context) error { return nil }
	}
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(ep), otlptracegrpc.WithInsecure())
	if err != nil {
		slog.Error("otel exporter", "err", err)
		return func(context.Context) error { return nil }
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceName(service))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown
}

// ServeHTTP serves /healthz and /metrics (plus any extra handlers) on addr.
func ServeHTTP(addr string, extra func(mux *http.ServeMux)) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", promhttp.Handler())
	if extra != nil {
		extra(mux)
	}
	go func() {
		slog.Info("http listening", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("http server", "err", err)
			os.Exit(1)
		}
	}()
}
