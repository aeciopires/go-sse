package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain lets the test binary act as the real program when GO_SSE_RUN_MAIN
// is set, so main() itself can be exercised end-to-end.
func TestMain(m *testing.M) {
	if os.Getenv("GO_SSE_RUN_MAIN") == "1" {
		os.Args = append([]string{"go-sse"}, strings.Fields(os.Getenv("GO_SSE_ARGS"))...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runMain(t *testing.T, args string, env ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0])
	cmd.Env = append(os.Environ(), append(env, "GO_SSE_RUN_MAIN=1", "GO_SSE_ARGS="+args)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestMainVersion(t *testing.T) {
	out, err := runMain(t, "-version")
	if err != nil || strings.TrimSpace(out) != "dev" {
		t.Errorf("go-sse -version = %q, %v", out, err)
	}
}

func TestMainExitsNonZeroOnError(t *testing.T) {
	out, err := runMain(t, "", "LOG_LEVEL=invalid")
	if err == nil || !strings.Contains(out, "go-sse: load config") {
		t.Errorf("expected failure, got err=%v output=%q", err, out)
	}
}
