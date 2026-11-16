// Command go-sse is a web application that demonstrates the Server-Sent
// Events (SSE) protocol with metrics, logs and traces.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aeciopires/go-sse/app/internal/app"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Run(ctx, app.Options{
		Args:    os.Args[1:],
		Getenv:  os.Getenv,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
	})
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go-sse:", err)
		os.Exit(1)
	}
}
