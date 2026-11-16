#!/usr/bin/env bash
# Shared helpers for check-deps.sh and install-deps.sh.
# Compatible with bash 3.2 (macOS default) and newer.

SUPPORTED_UBUNTU_VERSIONS="22.04 24.04 26.04"

if [ -t 1 ]; then
  C_RED=$'\033[31m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_RESET=$'\033[0m'
else
  C_RED=""; C_GREEN=""; C_YELLOW=""; C_RESET=""
fi

info()  { printf '%s[INFO]%s %s\n'  "$C_GREEN"  "$C_RESET" "$*"; }
warn()  { printf '%s[WARN]%s %s\n'  "$C_YELLOW" "$C_RESET" "$*" >&2; }
error() { printf '%s[ERROR]%s %s\n' "$C_RED"    "$C_RESET" "$*" >&2; }

# Repository root (directory that contains mise.toml).
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export REPO_ROOT

# Sets OS_FAMILY (ubuntu|macos), OS_VERSION and OS_ARCH, or exits when the
# platform is not supported.
detect_platform() {
  local kernel arch
  kernel="$(uname -s)"
  arch="$(uname -m)"

  case "$kernel" in
    Linux)
      if [ ! -r /etc/os-release ]; then
        error "Cannot read /etc/os-release: unsupported Linux distribution."
        exit 1
      fi
      # shellcheck disable=SC1091
      . /etc/os-release
      if [ "${ID:-}" != "ubuntu" ]; then
        error "Unsupported Linux distribution '${ID:-unknown}'. Supported: Ubuntu ${SUPPORTED_UBUNTU_VERSIONS}."
        exit 1
      fi
      case " $SUPPORTED_UBUNTU_VERSIONS " in
        *" ${VERSION_ID:-} "*) ;;
        *)
          error "Unsupported Ubuntu version '${VERSION_ID:-unknown}'. Supported: ${SUPPORTED_UBUNTU_VERSIONS}."
          exit 1
          ;;
      esac
      if [ "$arch" != "x86_64" ]; then
        error "Unsupported architecture '$arch' on Ubuntu. Supported: amd64 (x86_64)."
        exit 1
      fi
      OS_FAMILY="ubuntu"
      OS_VERSION="$VERSION_ID"
      OS_ARCH="amd64"
      ;;
    Darwin)
      case "$arch" in
        x86_64) OS_ARCH="amd64" ;;
        arm64)  OS_ARCH="arm64" ;;
        *)
          error "Unsupported architecture '$arch' on macOS. Supported: amd64, arm64."
          exit 1
          ;;
      esac
      OS_FAMILY="macos"
      OS_VERSION="$(sw_vers -productVersion 2>/dev/null || echo unknown)"
      ;;
    *)
      error "Unsupported operating system '$kernel'. Supported: Ubuntu ${SUPPORTED_UBUNTU_VERSIONS} (amd64) and macOS (amd64/arm64)."
      exit 1
      ;;
  esac
  export OS_FAMILY OS_VERSION OS_ARCH
}

# Makes a mise installed by https://mise.run visible in this shell.
load_mise() {
  if ! command -v mise >/dev/null 2>&1 && [ -x "$HOME/.local/bin/mise" ]; then
    PATH="$HOME/.local/bin:$PATH"
    export PATH
  fi
}

# Prints "docker compose" or "docker-compose", or nothing if neither exists.
compose_command() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    echo "docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
  fi
}
