package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	otelx "github.com/umbra-labs/agent-identity-control-plane/packages/go/otel"
	"github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	shutdown, err := otelx.Init(ctx, otelx.Config{
		ServiceName:  "controlplane-api",
		Environment:  getenv("UMBRA_ENV", "dev"),
		OtlpEndpoint: getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317"),
	}, logger)
	if err != nil {
		logger.Error("otel init failed", "err", err)
		os.Exit(1)
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	addr := ":" + getenv("PORT", "8080")
	mux := httpapi.Router(logger)

	srv := &http.Server{
		Addr:              addr,
		Handler:           otelhttp.NewHandler(mux, "controlplane-api.http"),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
