#!/usr/bin/env bash
# Checks that every tool needed to develop go-sse is installed.
# Usage: make check-deps   (or: ./scripts/check-deps.sh)
# Exit code: 0 when everything is present, 1 otherwise.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "$0")/lib.sh"

detect_platform
info "Platform: ${OS_FAMILY} ${OS_VERSION} (${OS_ARCH})"

missing=0

check_cmd() {
  local cmd="$1" hint="$2"
  if command -v "$cmd" >/dev/null 2>&1; then
    info "found $cmd: $(command -v "$cmd")"
  else
    error "missing $cmd -> $hint"
    missing=1
  fi
}

# Base system packages.
for cmd in git make curl tar; do
  check_cmd "$cmd" "run: make install-deps"
done
if [ "$OS_FAMILY" = "ubuntu" ]; then
  check_cmd gcc "run: make install-deps (needed by 'go test -race')"
fi

# Docker is installed separately (see REQUIREMENTS.md).
check_cmd docker "install Docker: https://docs.docker.com/engine/install/"
if [ -n "$(compose_command)" ]; then
  info "found $(compose_command)"
else
  error "missing Docker Compose -> https://docs.docker.com/compose/install/"
  missing=1
fi
if command -v docker >/dev/null 2>&1 && ! docker info >/dev/null 2>&1; then
  warn "docker is installed but the daemon is not reachable (is it running? is your user in the 'docker' group?)"
fi

# Tools pinned in mise.toml.
load_mise
check_cmd mise "run: make install-deps"
if command -v mise >/dev/null 2>&1; then
  cd "$REPO_ROOT"
  mise trust --quiet "$REPO_ROOT/mise.toml" >/dev/null 2>&1 || true
  missing_tools="$(mise ls --local --missing --no-header 2>/dev/null || true)"
  if [ -n "$missing_tools" ]; then
    error "tools from mise.toml not installed (run: make install-deps):"
    printf '%s\n' "$missing_tools" >&2
    missing=1
  else
    info "all tools pinned in mise.toml are installed:"
    mise ls --local --no-header 2>/dev/null || true
  fi
fi

if [ "$missing" -ne 0 ]; then
  error "Some dependencies are missing."
  exit 1
fi
info "All dependencies are installed."
