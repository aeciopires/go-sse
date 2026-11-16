# 5. Configuration

go-sse follows the [twelve-factor](https://12factor.net/config) idea: **all configuration comes from environment variables**. Think of them as the **knobs on the back of a radio** — you turn them before switching it on, and the code never has to change.

The parsing lives in [`app/internal/config/config.go`](../app/internal/config/config.go). Invalid values make the app **refuse to start** with a clear message (all problems are reported at once with `errors.Join`), instead of silently running with a wrong setting.

## Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN` (or `WARNING`), `ERROR` — case-insensitive |
| `SSE_EVENT_INTERVAL` | `2s` | Interval between `update` events ([Go duration](https://pkg.go.dev/time#ParseDuration): `500ms`, `5s`, `1m`) |
| `SHUTDOWN_DELAY` | `0s` | How long to keep serving after `/readyz` starts failing (use ~5s in Kubernetes) |
| `SHUTDOWN_TIMEOUT` | `15s` | Maximum time to wait for in-flight requests during shutdown |
| `OTEL_SERVICE_NAME` | `go-sse` | Service name in logs, traces and Loki's `service_name` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(unset)* | OTLP/HTTP endpoint, e.g. `http://otel-collector:4318`. **Setting it enables** trace and log export |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` / `..._LOGS_ENDPOINT` | *(unset)* | Signal-specific endpoints (also enable export) |
| `OTEL_SDK_DISABLED` | `false` | `true` forces OTLP export off |
| `OTEL_RESOURCE_ATTRIBUTES` | *(unset)* | Extra attributes, e.g. `deployment.environment.name=prod` |

Any other [OpenTelemetry SDK variable](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) (`OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_TRACES_SAMPLER`, ...) is honored by the exporters.

## Command-line flags

| Flag | Description |
|------|-------------|
| `-version` | Print the version and exit |
| `-healthcheck` | Call `http://127.0.0.1:$PORT/livez` and exit `0` (healthy) or `1`. Used by the Docker `HEALTHCHECK`, so the image does not need `curl`/`wget` |

## Examples

```bash
# Local run with verbose logs and faster events
cd app
LOG_LEVEL=debug SSE_EVENT_INTERVAL=500ms go run .

# Send traces/logs to a local collector
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 go run .

# Docker
docker run --rm -p 8080:8080 -e LOG_LEVEL=WARN aeciopires/go-sse:2.0.0
```

## How the version gets into the binary

`main.version` is set at build time:

```bash
go build -ldflags "-X main.version=2.0.0" .
```

The Makefile and the Dockerfile do this automatically using `VERSION` from [`app/Makefile`](../app/Makefile). It shows up in `/health`, in every log line, in the `sse_build_info` metric and on the web page.

Next: [Health checks and Kubernetes →](06-kubernetes.md)
