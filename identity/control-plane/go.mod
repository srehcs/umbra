module github.com/umbra-labs/agent-identity-control-plane

go 1.22

require (
  go.opentelemetry.io/otel v1.27.0
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.27.0
  go.opentelemetry.io/otel/sdk v1.27.0
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.52.0
  github.com/jackc/pgx/v5 v5.6.0
  github.com/google/uuid v1.6.0
)
