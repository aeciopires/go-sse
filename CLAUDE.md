# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Go code: always use the Modern Go Guidelines

Whenever **creating, modifying, reviewing/analyzing or testing Go code** in this repository, use the `modern-go-guidelines` skill (plugin from https://github.com/JetBrains/go-modern-guidelines, skill `modern-go-guidelines:use-modern-go`):

1. Before editing a Go file, run the skill's `list` command for that file (it resolves the Go version from `app/go.mod`, currently Go 1.27) and read the **full** output.
2. Treat the returned guidelines as authoritative (e.g. `encoding/json/v2` for new JSON code, stdlib `uuid`, typed atomics, `t.Context()`, `wg.Go`, `errors.AsType`, `new(value)`, method-aware `ServeMux` patterns). Call `explain <id>` before skipping a guideline that seems relevant.
3. When reviewing code, flag older idioms that a returned guideline replaces.

## Repository overview

This repo has two independent, versioned parts that are developed and released separately:

- `app/` — a Go web application demonstrating the Server-Sent Events (SSE) protocol, with Prometheus metrics, JSON logs, OpenTelemetry traces/logs and a Grafana dashboard.
- `helm-chart/` — a Helm chart (Helm 4 only) that deploys the `app/` Docker image to Kubernetes.

They share the app name (`go-sse`) but have independent version numbers (app version in `app/Makefile` `VERSION` → injected into the binary with `-X main.version`, image tag in `app/docker-compose.yaml`, `app/CHANGELOG.md`; chart version in `helm-chart/Chart.yaml`). Don't assume a bump to one implies a bump to the other.

Beginner-oriented documentation lives in `docs/` (keep it in sync when behavior changes).

## Commands

Tool versions are pinned in the root `mise.toml` (Go, Helm 4, helm-docs, kubectl, Trivy, golangci-lint). Supported dev platforms: Ubuntu 22.04/24.04/26.04 amd64 and macOS amd64/arm64. Docker/Compose are installed separately. Makefiles must stay compatible with GNU Make 3.81 (macOS) — no `.ONESHELL`, `SHELL := /bin/bash`.

### Root

```bash
make check-deps      # scripts/check-deps.sh
make install-deps    # scripts/install-deps.sh (apt/xcode + mise + mise install)
```

### App (`cd app`)

```bash
make run             # build + run on :8080 (LOG_LEVEL=debug make run)
make test            # go test -race with coverage; fails below 90% total
make lint            # golangci-lint (app/.golangci.yml)
make fmt / make vet
make image-build     # docker build aeciopires/go-sse:$(VERSION)
make scan            # Trivy image scan (fails on fixable HIGH/CRITICAL)
make image           # multi-platform build + push + scan
make compose-up      # app + OTel Collector + Prometheus + Loki + Tempo + Jaeger + Grafana
make compose-down / make compose-logs
make dashboard       # regenerate dashboard JSON from dashboard/generate.py and copy into the chart
```

Run a single test: `go test -race -run TestSSEStream ./internal/server/`.

When changing app behavior, bump `VERSION` in `app/Makefile`, the build arg/image tag in `app/docker-compose.yaml`, and add an entry to `app/CHANGELOG.md` (Keep a Changelog format).

### Helm chart (`cd helm-chart`)

```bash
make lint            # helm lint --strict (alpine/helm:4.x in Docker)
make template        # render manifests locally (Docker)
make gen-docs        # regenerate README.md (helm-docs in Docker)
make sync-dashboard  # copy ../app/dashboard/go-sse-dashboard.json into dashboards/
make dry-run / install / uninstall   # use local helm; `check-helm` refuses anything but Helm 4
```

`helm-chart/README.md` is generated — edit `README.md.gotmpl` and/or the `# --` comments in `values.yaml`, then run `make gen-docs`.

Chart releases follow `CONTRIBUTING.md` (bump `Chart.yaml`, regenerate docs, tag so CI publishes).

## Architecture

### App (`app/`)

- `main.go` — only wires `signal.NotifyContext` and calls `internal/app.Run`.
- `internal/app` — parses flags (`-version`, `-healthcheck`), loads config, sets up telemetry, logger, metrics and server, listens and runs until the context is canceled; `Healthcheck` probes `/livez`.
- `internal/config` — env vars → `Config` (`PORT`, `LOG_LEVEL` default INFO, `SSE_EVENT_INTERVAL`, `SHUTDOWN_DELAY`, `SHUTDOWN_TIMEOUT`, `OTEL_SERVICE_NAME`; OTLP export enabled when an `OTEL_EXPORTER_OTLP_*ENDPOINT` is set and `OTEL_SDK_DISABLED` is not true). Errors are joined.
- `internal/logging` — `slog` JSON handler (adds `service`, `version`, and `trace_id`/`span_id` from the context) fanned out with `slog.NewMultiHandler` to the OTLP log bridge (level-filtered).
- `internal/telemetry` — OTel tracer + logger providers with OTLP/HTTP exporters (configured by standard `OTEL_*` env vars; exporter constructors are package vars for tests), W3C propagator, no-op provider when disabled.
- `internal/metrics` — dedicated Prometheus registry: `sse_http_requests_total{path,method}`, `sse_http_request_duration_seconds{path,method,code}`, `sse_active_connections{endpoint}`, `sse_events_sent_total{event_type}`, `sse_event_duration_seconds{event_type}`, `sse_session_duration_seconds`, `sse_build_info`, Go/process collectors. `/metrics` serves OpenMetrics so `Observe` can attach `trace_id` exemplars.
- `internal/server` — `ServeMux` with method patterns: `GET /{$}` (index, exact match), `GET /static/`, `GET /sse`, `GET /health`, `GET /livez`, `GET /readyz` (503 unless ready), `GET /metrics`. Each route is wrapped by `instrument` (metrics, `statusRecorder` implements `Unwrap`) and optionally `otelhttp`; everything by `securityHeaders` (CSP etc.). `Serve` handles graceful shutdown: readiness false → `ShutdownDelay` → cancel `streamsCtx` (ends SSE streams) → `http.Server.Shutdown`.
- `internal/server/sse.go` — `handleSSE`: sets SSE headers + `X-Accel-Buffering: no`, lifts the write deadline and flushes via `http.ResponseController`, joins request ctx with `streamsCtx` (`context.AfterFunc`), honors `Last-Event-ID`, sends `welcome` (with `retry:`) then `update` every `EventInterval` (`time.Tick`). `sendEvent` creates a new root span per event linked to the connection span, writes the event with a single `Write` (`writeEvent`), flushes, records metrics. Connection count is an `atomic.Int64`; client IDs are stdlib `uuid.NewV7()`; payloads use `encoding/json/v2`.
- `web/` — `templates/index.html`, `static/{app.js,theme.js,style.css,favicon.svg}` embedded with `go:embed`. Material Design 3 UI (baseline color tokens, nav rail + drawer), dark default with light toggle stored in `localStorage` (`theme.js` runs in `<head>`). DOM is built with `textContent` only.
- `observability/` — configs for the OTel Collector (traces → Tempo + Jaeger, logs → Loki OTLP), Prometheus (remote-write receiver + exemplar storage), Loki, Tempo (metrics-generator span-metrics/service-graphs), Grafana provisioning (data sources with metrics↔logs↔traces links, dashboard provider).
- `dashboard/` — `go-sse-dashboard.json` is generated by `generate.py` (data sources are dashboard variables); also copied to `helm-chart/dashboards/`.

Tests: every package has `*_test.go`; `main_test.go` re-executes the test binary to cover `main()`. Keep total coverage ≥ 90%.

### Helm chart (`helm-chart/`)

Templates: `deployment.yaml` (env from values: `logLevel`, `sse.eventInterval`, `shutdown.*`, `otel.*`, extra `env`; startup/liveness on `/livez`, readiness on `/readyz`; hardened `podSecurityContext`/`securityContext` with UID 10001 and read-only root FS), `service.yaml`, `ingress.yaml`, `hpa.yaml`, `pdb.yaml`, `servicemonitor.yaml`, `grafana-dashboard.yaml` (ConfigMap from `dashboards/` when `grafanaDashboard.enabled`), `cert-manager-issuer.yaml`, `gcp-certificate.yaml`, `gcp-backend-frontend-config.yaml`, `extra-manifests.yaml`, helpers in `_helpers.tpl`. Optional features are gated by `values.yaml` flags (`autoscaling.enabled`, `podDisruptionBudget.enabled`, `serviceMonitor.enabled`, `grafanaDashboard.enabled`, `otel.enabled`, `ingress.enabled` + GCP/cert-manager flags).

## Contribution workflow

`CONTRIBUTING.md` documents a fork/upstream-remote/feature-branch/PR workflow (no direct pushes to `main`). `REQUIREMENTS.md` covers tool setup via `mise` and Docker installation.
