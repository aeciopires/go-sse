# Requirements

<!-- TOC -->

- [Supported platforms](#supported-platforms)
- [Quick install](#quick-install)
- [Tool versions](#tool-versions)
- [Docker](#docker)
  - [Ubuntu](#ubuntu)
  - [macOS](#macos)
- [mise](#mise)
- [Manual installation details](#manual-installation-details)
  - [Base packages](#base-packages)
  - [Tools managed by mise](#tools-managed-by-mise)
- [Troubleshooting](#troubleshooting)

<!-- TOC -->

# Supported platforms

| OS | Versions | Architectures |
|----|----------|---------------|
| Ubuntu | 22.04, 24.04, 26.04 | amd64 |
| macOS | current releases | amd64 (Intel), arm64 (Apple Silicon) |

# Quick install

```bash
# 1. Install Docker and Docker Compose (see the Docker section below)
# 2. Install everything else:
make install-deps
# 3. Verify:
make check-deps
```

`make install-deps` ([scripts/install-deps.sh](scripts/install-deps.sh)):

1. Ubuntu: installs `build-essential ca-certificates curl git gnupg make tar gzip unzip` with `apt` (`gcc` is needed by `go test -race`).
   macOS: checks the Xcode Command Line Tools (starts the installer if missing).
2. Installs [mise](https://mise.jdx.dev) if missing (Homebrew on macOS when available, otherwise `https://mise.run`).
3. Runs `mise install` to install the tools pinned in [mise.toml](mise.toml).

`make check-deps` ([scripts/check-deps.sh](scripts/check-deps.sh)) checks the base packages, Docker, Docker Compose, mise and every pinned tool, and exits with code `1` if anything is missing.

The same targets are available in `app/` and `helm-chart/` (`make -C app check-deps`).

# Tool versions

Pinned in [mise.toml](mise.toml):

| Tool | Version | Used for |
|------|---------|----------|
| [Go](https://go.dev) | 1.27.4 | building and testing the app |
| [Helm](https://helm.sh) | 4.3.2 | deploying the chart (**Helm 3 is not supported**) |
| [helm-docs](https://github.com/norwoodj/helm-docs) | 1.14.2 | generating `helm-chart/README.md` |
| [kubectl](https://kubernetes.io/docs/reference/kubectl/) | 1.37.2 | talking to Kubernetes |
| [Trivy](https://trivy.dev) | 0.76.2 | scanning the Docker image and manifests |
| [golangci-lint](https://golangci-lint.run) | 2.15.0 | Go linting |

Container images used by `app/docker-compose.yaml`:

| Image | Version |
|-------|---------|
| `golang` (builder) / `alpine` (runtime) | 1.27.4-alpine3.24 / 3.24.4 |
| `grafana/grafana` | 13.3.1 |
| `grafana/loki` | 3.7.10 |
| `grafana/tempo` | 3.1.1 |
| `prom/prometheus` | v3.16.0 |
| `otel/opentelemetry-collector-contrib` | 0.165.0 |
| `jaegertracing/jaeger` | 2.23.0 |

# Docker

Docker Engine and the Docker Compose plugin are **not** installed by `make install-deps`.

## Ubuntu

```bash
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
# Use docker without sudo (log out and in again afterwards)
sudo usermod -aG docker "$USER"
docker compose version
```

Reference: https://docs.docker.com/engine/install/ubuntu/ and https://docs.docker.com/engine/install/linux-postinstall/

## macOS

Install [Docker Desktop](https://docs.docker.com/desktop/setup/install/mac-install/) (choose *Apple Silicon* or *Intel chip*). It includes Docker Compose.

For more information: https://docs.docker.com and http://blog.aeciopires.com/primeiros-passos-com-docker

# mise

[mise](https://mise.jdx.dev) installs and switches tool versions per project (it replaces asdf in this repository). After `make install-deps`, activate it in your shell so the pinned versions are used automatically inside the repository:

```bash
# bash
echo 'eval "$(mise activate bash)"' >> ~/.bashrc
# zsh (macOS default)
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc
```

Useful commands:

```bash
mise install          # install the versions from mise.toml
mise ls --local       # show the versions in use
mise upgrade          # upgrade within the pinned constraints
mise exec -- helm version   # run a command with the pinned tools without activation
```

# Manual installation details

## Base packages

Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y build-essential ca-certificates curl git gnupg make tar gzip unzip
```

macOS:

```bash
xcode-select --install
```

## Tools managed by mise

```bash
curl -fsSL https://mise.run | sh     # or: brew install mise
cd go-sse
mise trust
mise install
```

To update a tool, change its version in `mise.toml` and run `mise install`.

# Troubleshooting

- **`Helm 4 is required (found: 'v3...')`**: another Helm is earlier in your `PATH`. Activate mise in your shell or run `mise exec -- make install`.
- **`go test -race` fails with `cgo` errors on Ubuntu**: install `build-essential` (`make install-deps` does it).
- **Docker permission denied**: add your user to the `docker` group and log in again.
- **macOS ships GNU Make 3.81**: the Makefiles are compatible with it; no need to install a newer make.
