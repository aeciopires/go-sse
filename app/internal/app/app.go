// Package app wires configuration, logging, telemetry, metrics and the HTTP
// server together. It is kept separate from package main so it can be tested.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/aeciopires/go-sse/app/internal/config"
	"github.com/aeciopires/go-sse/app/internal/logging"
	"github.com/aeciopires/go-sse/app/internal/metrics"
	"github.com/aeciopires/go-sse/app/internal/server"
	"github.com/aeciopires/go-sse/app/internal/telemetry"
)

const (
	healthcheckTimeout       = 3 * time.Second
	telemetryShutdownTimeout = 5 * time.Second
)

// Options are the process-level inputs of Run.
type Options struct {
	Args    []string
	Getenv  func(string) string
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
}

// Run starts the application and blocks until ctx is canceled.
//
// Flags:
//
//	-version      print the version and exit
//	-healthcheck  probe http://127.0.0.1:$PORT/livez and exit (for Docker HEALTHCHECK)
func Run(ctx context.Context, opts Options) error {
	fs := flag.NewFlagSet("go-sse", flag.ContinueOnError)
	fs.SetOutput(opts.Stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	healthcheck := fs.Bool("healthcheck", false, "probe the local /livez endpoint and exit")
	if err := fs.Parse(opts.Args); err != nil {
		return err
	}

	if *showVersion {
		_, err := fmt.Fprintln(opts.Stdout, opts.Version)
		return err
	}

	cfg, err := config.Load(opts.Getenv, opts.Version)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if *healthcheck {
		return Healthcheck(ctx, "http://127.0.0.1"+cfg.Addr+"/livez")
	}

	tel, err := telemetry.Setup(ctx, telemetry.Options{
		Enabled:        cfg.TelemetryEnabled,
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
	})
	if err != nil {
		return fmt.Errorf("setup telemetry: %w", err)
	}

	var extra []slog.Handler
	if tel.LogHandler != nil {
		extra = append(extra, tel.LogHandler)
	}
	logger := logging.New(opts.Stdout, logging.Options{
		Level:          cfg.LogLevel,
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		Extra:          extra,
	})
	slog.SetDefault(logger)

	srv, err := server.New(server.Options{
		Config:         cfg,
		Logger:         logger,
		Metrics:        metrics.New(cfg.ServiceVersion),
		TracerProvider: tel.TracerProvider,
	})
	if err != nil {
		return errors.Join(err, shutdownTelemetry(ctx, tel))
	}

	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen on %s: %w", cfg.Addr, err), shutdownTelemetry(ctx, tel))
	}

	logger.InfoContext(ctx, "starting go-sse",
		"log_level", cfg.LogLevel.String(),
		"event_interval", cfg.EventInterval.String(),
		"telemetry_enabled", cfg.TelemetryEnabled,
	)
	err = srv.Serve(ctx, ln)
	logger.InfoContext(ctx, "go-sse stopped")
	return errors.Join(err, shutdownTelemetry(ctx, tel))
}

func shutdownTelemetry(ctx context.Context, tel *telemetry.Telemetry) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), telemetryShutdownTimeout)
	defer cancel()
	if err := tel.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown telemetry: %w", err)
	}
	return nil
}

// Healthcheck performs a GET on url and fails unless it returns 200 OK.
// It lets the container image run health checks without curl/wget.
func Healthcheck(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: unexpected status %s", resp.Status)
	}
	return nil
}
