# go-sse Grafana Dashboard

<!-- TOC -->

- [Overview](#overview)
- [Panels](#panels)
- [Variables](#variables)
- [How to use](#how-to-use)
  - [Docker Compose (automatic)](#docker-compose-automatic)
  - [Kubernetes (Helm chart)](#kubernetes-helm-chart)
  - [Manual import](#manual-import)
- [Editing the dashboard](#editing-the-dashboard)
- [Troubleshooting](#troubleshooting)

<!-- TOC -->

## Overview

[`go-sse-dashboard.json`](go-sse-dashboard.json) (title *go-sse - Metrics, Logs and Traces*, uid `go-sse-monitoring`) shows the three observability signals of go-sse in one place:

| Signal | Data source | Origin |
|--------|-------------|--------|
| Metrics | Prometheus | `/metrics` of go-sse + Tempo span metrics |
| Logs | Loki | JSON logs (OTLP or collected from stdout) |
| Traces | Tempo | OpenTelemetry spans |

See [docs/04-observability.md](../../docs/04-observability.md) for the concepts.

## Panels

| Row | Panels |
|-----|--------|
| Overview | Active SSE connections, events/s, HTTP requests/s, HTTP 5xx ratio, error logs in range, version |
| Metrics (Prometheus) | HTTP request rate by path, HTTP latency p95 (exemplars), active connections, events rate by type, event types distribution, event processing duration p50/p95/p99/avg (exemplars), SSE session duration, goroutines, heap, CPU |
| Logs (Loki) | Log volume by level, application logs (level + text filter), warnings and errors |
| Traces (Tempo) | Span rate and p95 latency by operation (span metrics), recent traces, error traces, service graph request rate |

Exemplars (diamonds on latency graphs) and the `trace_id` field in logs link directly to Tempo.

## Variables

| Variable | Description |
|----------|-------------|
| `prometheus_ds`, `loki_ds`, `tempo_ds` | Data sources (pick yours when importing) |
| `job` | Prometheus job(s) exposing `sse_build_info` |
| `service` | Service name in Loki/Tempo (`service_name` / `service.name`), default `go-sse` |
| `level` | Log levels to show (`debug`, `info`, `warn`, `error`) |
| `search` | Free text filter for the logs panel |

## How to use

### Docker Compose (automatic)

```bash
cd app
make compose-up
```

Open http://localhost:3000 (admin / admin) → **Dashboards → go-sse → go-sse - Metrics, Logs and Traces**. Data sources and the dashboard are provisioned from [`../observability/grafana/provisioning`](../observability/grafana/provisioning).

### Kubernetes (Helm chart)

```bash
helm upgrade --install go-sse ./helm-chart -n go-sse --set grafanaDashboard.enabled=true
```

A ConfigMap labeled `grafana_dashboard: "1"` is created for the Grafana sidecar (kube-prometheus-stack / Grafana Helm chart).

### Manual import

Grafana → **Dashboards → New → Import** → upload `go-sse-dashboard.json` → choose the Prometheus, Loki and Tempo data sources.

Requirements:

- Prometheus with `--enable-feature=exemplar-storage` for exemplars, and scraping go-sse with the OpenMetrics format.
- Loki 3.x with `discover_log_levels` (default) for the `detected_level` filters.
- Tempo with the metrics-generator (`span-metrics`, `service-graphs`) remote-writing to Prometheus for the span metric panels.

## Editing the dashboard

The JSON is **generated** — edit [`generate.py`](generate.py) and run:

```bash
cd app
make dashboard   # regenerates the JSON and copies it to helm-chart/dashboards/
```

Changes made in the Grafana UI are allowed (`allowUiUpdates`) but are lost on restart unless you port them to `generate.py`.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Metric panels empty | http://localhost:9090/targets — `go-sse` target must be UP |
| Log panels empty | the app needs `OTEL_EXPORTER_OTLP_ENDPOINT`; check `docker compose logs otel-collector loki` |
| Trace panels empty | open the web page and click **Connect SSE** to produce spans; check `docker compose logs tempo` |
| Span metric panels empty | Prometheus must run with `--web.enable-remote-write-receiver` |
| "Active SSE connections" is 0 | normal when no browser is connected |
