FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
# Builder stage

WORKDIR /app

# REQUIRED for multi-arch builds
ARG TARGETOS
ARG TARGETARCH

COPY . .

RUN go mod download

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -o /out/go-sse main.go

#----------------------------------    

FROM alpine:3.23.3

# About image
LABEL maintainer="Aécio Pires" \
      date_create="02/12/2026" \
      version="1.0.0" \
      description="A web application for testing Server-Sent Events (SSE) protocol using Golang." \
      licensce="GPL-3.0"

WORKDIR /app

# Copy files to image
COPY --from=builder /out/go-sse /app/go-sse
COPY templates/ /app/templates
COPY static/ /app/static

# Exposing port of application
EXPOSE 8080

CMD ["./go-sse"]
