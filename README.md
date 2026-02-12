# go-sse

<!-- TOC -->

- [go-sse](#go-sse)
- [Introduction](#introduction)
- [Architecture](#architecture)
  - [Features](#features)
    - [API Endpoints](#api-endpoints)
    - [Metrics Documentation](#metrics-documentation)
- [Running using Docker](#running-using-docker)
- [Running locally without Docker](#running-locally-without-docker)
- [Build image](#build-image)
- [Developers](#developers)
- [License](#license)

<!-- TOC -->

# Introduction

A web application for testing and demonstrating the Server-Sent Events (SSE) protocol using Golang.

Server-Sent Events (SSE) is a lightweight, unidirectional protocol for streaming real-time text data from a server to a client over a single, long-lived HTTP connection. Using the `text/event-stream` format, it enables efficient server-to-browser updates (e.g., dashboards, notifications) with built-in auto-reconnection.

References:
- [Server-Sent Events - HTML Living Standard](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [MDN Web Docs - Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [RFC 6797 - HTTP Over TLS](https://www.rfc-editor.org/rfc/rfc6797)

It implements a Backend in [Go](https://go.dev/) serving a Frontend, [Prometheus](https://prometheus.io) metrics, and [Docker](https://docs.docker.com/get-started/) configurations.

# Architecture

The application follows a clean, modular architecture:

- **Frontend**: A server-side rendered [HTML](https://www.w3schools.com/html/) template served by Go with [Vanilla JavaScript](http://vanilla-js.com/) to consume SSE events in real-time.
- **Backend**: Written in [Go](https://go.dev/), exposing a `/sse` endpoint that streams Server-Sent Events with proper headers and persistent connections.
- **Observability**: Uses [prometheus/client_golang](https://github.com/prometheus/client_golang) to expose system and application metrics.

## Features

### SSE Protocol Compliance

The application correctly implements the Server-Sent Events specification with:

- **Persistent HTTP Connection**: Long-lived TCP connection maintained between client and server
- **No Caching**: `Cache-Control: no-cache` header set
- **No Compression**: Uncompressed streaming for real-time delivery
- **No Buffering**: Server-side flushing with `http.Flusher` interface
- **Proper Streaming Headers**: 
  - `Content-Type: text/event-stream`
  - `Connection: keep-alive`
  - `HTTP/1.1 200 OK`
- **Built-in Auto-reconnection**: Browser automatically reconnects on connection loss

### Application Features

- **Real-time Event Streaming**: Server sends simulated system metrics every 2 seconds
- **Event Types**: Welcome message, periodic updates with simulated system data
- **Connection Management**: Track active SSE connections
- **Frontend Dashboard**: Display received events, metrics, and connection status
- **Prometheus Metrics**: 
  - HTTP request counts
  - Active SSE connections
  - Events sent counter
  - Event processing duration

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Frontend HTML interface |
| `/sse` | GET | Server-Sent Events stream |
| `/health` | GET | Health check endpoint |
| `/metrics` | GET | Prometheus metrics endpoint |
| `/static/*` | GET | Static assets (CSS, JavaScript) |

### Metrics Documentation

The application exposes the following Prometheus metrics:

- `sse_http_requests_total` - Total HTTP requests by path and method
- `sse_active_connections` - Number of active SSE connections
- `sse_events_sent_total` - Total events sent via SSE endpoint
- `sse_event_duration_seconds` - Event processing duration histogram

# Running using Docker

## Prerequisites

- [Docker](https://docs.docker.com/get-started/)

## Build and Run

```bash
cd app

# Using Docker Compose (includes Prometheus and Grafana)
docker-compose up -d

# Check logs
docker-compose logs -f go-sse

# Open the application
# Frontend: http://localhost:8080
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000 (admin / admin)

# Stop services
docker-compose down
```

## Using Docker Image

```bash
docker build -t aeciopires/go-sse:1.0.0 .

docker run -d -p 8080:8080 --name go-sse aeciopires/go-sse:1.0.0

docker logs -f go-sse

docker stop go-sse
```

# Running locally without Docker

## Prerequisites

- [Go 1.25.0](https://go.dev/doc/install) or later
- [Make](https://www.gnu.org/software/make/)

## Installation and Execution

1. Install dependencies:

```bash
cd app
go mod download
go mod tidy
```

2. Run the application:

```bash
go run main.go
```

3. Access the application:

Open your browser and navigate to `http://localhost:8080`

The application will:
- Serve the frontend on `/`
- Stream events on `/sse`
- Expose metrics on `/metrics`
- Provide health check on `/health`

## Using Makefile

```bash
cd app

# Build the binary
make build

# Build and run Docker image
make up

# View logs
make logs

# Stop the container
make down

# Recreate container
make recreate
```

# Build image

To build a Docker image with multi-platform support:

```bash
cd app

# Build for multiple platforms and push to Docker Hub
make image

# You will be prompted for:
# - Docker Hub username
# - Docker Hub password
```

The command will build and push images for:
- `linux/amd64`
- `linux/arm/v6`
- `linux/arm/v7`
- `linux/arm64`

# Developers

See the [CONTRIBUTING.md](CONTRIBUTING.md) file for development guidelines and instructions on how to contribute to this project.

# License

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.
