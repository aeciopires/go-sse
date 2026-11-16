# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27.4
ARG ALPINE_VERSION=3.24.4

#---------------------------------- Builder stage
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine3.24 AS builder

WORKDIR /src

# REQUIRED for multi-arch builds
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/go-sse .

#---------------------------------- Runtime stage
FROM alpine:${ALPINE_VERSION}

ARG VERSION=dev

LABEL maintainer="Aécio Pires" \
      org.opencontainers.image.title="go-sse" \
      org.opencontainers.image.description="A web application for testing Server-Sent Events (SSE) protocol using Golang." \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/aeciopires/go-sse" \
      org.opencontainers.image.licenses="GPL-3.0"

# Apply security fixes from the Alpine repository and create an unprivileged user.
RUN apk upgrade --no-cache \
    && addgroup -S -g 10001 gosse \
    && adduser -S -D -H -u 10001 -G gosse gosse

# Templates and static files are embedded in the binary.
COPY --from=builder /out/go-sse /app/go-sse

USER 10001:10001
WORKDIR /app

ENV PORT=8080 \
    LOG_LEVEL=INFO

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/app/go-sse", "-healthcheck"]

ENTRYPOINT ["/app/go-sse"]
