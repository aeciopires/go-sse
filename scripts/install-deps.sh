#!/usr/bin/env bash
# Installs the tools needed to develop go-sse.
#
# Supported platforms:
#   - Ubuntu 22.04, 24.04 and 26.04 (amd64)
#   - macOS (amd64 and arm64)
#
# What it does:
#   1. installs base system packages (apt on Ubuntu, Xcode Command Line Tools on macOS)
#   2. installs mise (https://mise.jdx.dev) if it is not installed
#   3. installs every tool pinned in mise.toml (Go, Helm 4, helm-docs, kubectl,
#      Trivy, golangci-lint)
#
# Docker and Docker Compose are NOT installed by this script (see REQUIREMENTS.md).
#
# Usage: make install-deps   (or: ./scripts/install-deps.sh)
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "$0")/lib.sh"

detect_platform
info "Platform: ${OS_FAMILY} ${OS_VERSION} (${OS_ARCH})"

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

install_ubuntu_packages() {
  info "Installing base packages with apt..."
  $SUDO apt-get update -y
  # build-essential provides gcc, required by 'go test -race' (cgo).
  $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    build-essential ca-certificates curl git gnupg make tar gzip unzip
}

install_macos_packages() {
  if ! xcode-select -p >/dev/null 2>&1; then
    warn "Xcode Command Line Tools (git, make, clang) are required."
    info "Starting the installer; re-run 'make install-deps' after it finishes."
    xcode-select --install || true
    exit 1
  fi
  info "Xcode Command Line Tools found: $(xcode-select -p)"
}

install_mise() {
  load_mise
  if command -v mise >/dev/null 2>&1; then
    info "mise already installed: $(mise --version)"
    return
  fi
  if [ "$OS_FAMILY" = "macos" ] && command -v brew >/dev/null 2>&1; then
    info "Installing mise with Homebrew..."
    brew install mise
  else
    info "Installing mise with the official installer (https://mise.run)..."
    curl -fsSL https://mise.run | sh
  fi
  load_mise
  if ! command -v mise >/dev/null 2>&1; then
    error "mise installation failed."
    exit 1
  fi
  info "mise installed: $(mise --version)"
  warn "Activate mise in your shell (then open a new terminal):"
  case "${SHELL:-}" in
    */zsh)  echo "  echo 'eval \"\$(mise activate zsh)\"' >> ~/.zshrc" ;;
    */fish) echo "  echo 'mise activate fish | source' >> ~/.config/fish/config.fish" ;;
    *)      echo "  echo 'eval \"\$(mise activate bash)\"' >> ~/.bashrc" ;;
  esac
}

install_tools() {
  cd "$REPO_ROOT"
  info "Installing tools pinned in mise.toml..."
  mise trust --quiet "$REPO_ROOT/mise.toml"
  mise install --yes
  mise ls --local --no-header || true
}

case "$OS_FAMILY" in
  ubuntu) install_ubuntu_packages ;;
  macos)  install_macos_packages ;;
esac
install_mise
install_tools

if ! command -v docker >/dev/null 2>&1 || [ -z "$(compose_command)" ]; then
  warn "Docker / Docker Compose not found. Install them separately: see REQUIREMENTS.md."
fi

info "Done. Run 'make check-deps' to verify the installation."
