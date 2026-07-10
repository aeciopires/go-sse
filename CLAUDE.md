# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository overview

This repo has two independent, versioned parts that are developed and released separately:

- `app/` — a single-file Go web application (`main.go`) demonstrating the Server-Sent Events (SSE) protocol, with Prometheus metrics and a Grafana dashboard.
- `helm-chart/` — a Helm chart that deploys the `app/` Docker image to Kubernetes.

They share the app name (`go-sse`) but have independent version numbers (app version in `app/Makefile` / `app/Dockerfile` / `app/CHANGELOG.md`; chart version in `helm-chart/Chart.yaml`). Don't assume a bump to one implies a bump to the other.

## Commands

All commands below assume the stated working directory — the two Makefiles are separate and not interchangeable.

### App (`cd app`)

```bash
# Run locally without Docker (Go 1.25+ required)
go mod download
go run main.go                 # serves on :8080

# Build binary (also updates deps via `go get -u`) and run it
make build

# Docker Compose stack: app + Prometheus (:9090) + Grafana (:3000, admin/admin)
docker-compose up -d
docker-compose logs -f go-sse
docker-compose down

# Single-container image workflow (uses aeciopires/go-sse:<VERSION> from the Makefile)
make up      # docker run + logs
make logs
make down
make recreate

# Multi-platform build + push to Docker Hub + Trivy scan (prompts for Docker Hub creds)
make image
```

There are no Go tests in this repo (`*_test.go` does not exist yet). There is no linter/formatter target — use `go vet ./...` and `gofmt` conventionally if needed.

When changing app behavior, bump `VERSION` in `app/Makefile`, the image `version` label in `app/Dockerfile`, and add an entry to `app/CHANGELOG.md` (Keep a Changelog format).

### Helm chart (`cd helm-chart`)

All chart commands run through Dockerized `helm`/`helm-docs` via the Makefile — no local Helm install is required for these targets:

```bash
make lint        # helm lint .
make template     # render manifests locally (namespace: go-sse)
make dry-run      # helm install --dry-run --debug
make install      # helm upgrade --install go-sse . -n go-sse --create-namespace
make uninstall    # helm uninstall go-sse -n go-sse
make package      # package chart into ./packages
make gen-docs     # regenerate helm-chart/README.md from README.md.gotmpl + values.yaml + Chart.yaml
```

`helm-chart/README.md` is generated — edit `helm-chart/README.md.gotmpl` and/or the `# --` comments in `values.yaml`, then run `make gen-docs`, rather than hand-editing `README.md`.

Chart releases follow the process in `CONTRIBUTING.md` and are cut with the `release-chart` skill (bump `Chart.yaml`, regenerate docs, tag so CI publishes). Chart version and app version in `Chart.yaml` follow semver independently per the comments in that file.

## Architecture

### App (`app/main.go`)

Single-file `net/http` server (no framework, no router library) with these routes:

| Route | Purpose |
|---|---|
| `/` | Renders `templates/index.html` (server-side template, vanilla-JS frontend in `static/app.js`) |
| `/sse` | The SSE stream — see below |
| `/health` | Static JSON health check |
| `/metrics` | `promhttp.Handler()` — Prometheus scrape endpoint |
| `/static/*` | File server over `static/` |

`/sse` (`handleSSE`) is the core of the app: it sets the three required SSE headers (`Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`), sends a `welcome` event, then loops on a 2-second `time.Ticker` sending simulated `update` events (fake CPU/memory/connection stats) via `sendSSEEvent`, until the request context is cancelled (client disconnect), flushing after every write with `http.Flusher`. Connection count is tracked in the package-level `connManager` (`ConnectionManager`, mutex-guarded increment/decrement/getCount) — this drives both the `connections` field in each event payload and the `sse_active_connections` gauge.

Four Prometheus metrics are registered in `init()` and updated inline in the handlers: `sse_http_requests_total` (counter, labeled path+method, recorded via `recordHTTPRequest` in every handler), `sse_active_connections` (gauge), `sse_events_sent_total` (counter, labeled event_type), `sse_event_duration_seconds` (histogram, labeled event_type — observed once per event and again for the whole session on disconnect).

`app/dashboard/go-sse-dashboard.json` is a Grafana dashboard built against these four metrics; `app/prometheus.yml` configures the scrape job used by `docker-compose.yaml`.

### Helm chart (`helm-chart/`)

Standard chart layout under `templates/`: `deployment.yaml`, `service.yaml`, `ingress.yaml`, `hpa.yaml`, `pdb.yaml`, `servicemonitor.yaml`, `cert-manager-issuer.yaml`, `gcp-certificate.yaml`, `gcp-backend-frontend-config.yaml`, `extra-manifests.yaml` (passthrough for arbitrary manifests via `values.yaml`'s `extraManifests`), plus shared naming/label helpers in `_helpers.tpl`.

Notable optional features gated by `values.yaml` flags, each tied to a template file: `autoscaling.enabled` → HPA, `podDisruptionBudget.enabled` → PDB, `serviceMonitor.enabled` → Prometheus Operator ServiceMonitor, `ingress.enabled` (+ `createGcpManagedCertificate` / `createCertManagerIssuer` / `service.createGcpBackendAndFrontendConfig`) → GCP- or cert-manager-specific ingress resources. Cloud-specific templates (GCP cert/backend-config, cert-manager issuer) only render when their corresponding flag is true, so the chart supports GKE, generic ingress-nginx/ALB, and plain ClusterIP/NodePort setups from one values file.

## Contribution workflow

`CONTRIBUTING.md` documents a standard fork/upstream-remote/feature-branch/PR workflow (no direct pushes to `main`). `REQUIREMENTS.md` covers local tool setup via `asdf` (Go 1.25.0, Helm 3.19.0, helm-docs 1.14.2, kubectl).
