# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-02-12

### Added
- Initial release of go-sse application
- Server-Sent Events (SSE) endpoint at `/sse`
- Proper SSE protocol compliance with required headers:
  - `Content-Type: text/event-stream`
  - `Cache-Control: no-cache`
  - `Connection: keep-alive`
- Real-time event streaming with periodic updates
- Frontend dashboard with event display
- Connection management and tracking
- Prometheus metrics integration:
  - HTTP request counting
  - Active connection monitoring
  - Event counter
  - Event processing duration tracking
- Health check endpoint at `/health`
- Docker support with multi-platform builds
- Docker Compose configuration with Prometheus and Grafana
- Helm chart for Kubernetes deployment
- Comprehensive documentation and examples
