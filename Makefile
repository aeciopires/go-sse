# go-sse repository Makefile.
# The application (app/) and the Helm chart (helm-chart/) have their own
# Makefiles; this one handles the developer machine setup and delegates.
# Works with GNU Make 3.81+ (macOS default). Run `make help` to list the targets.

SHELL := /bin/bash
.DEFAULT_GOAL := help

.PHONY: help check-deps install-deps test lint scan chart-lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  More targets: make -C app help | make -C helm-chart help"

check-deps: ## Check that all required software is installed (Ubuntu 22.04/24.04/26.04 amd64, macOS amd64/arm64)
	@./scripts/check-deps.sh

install-deps: ## Install required software with apt/xcode + mise (Docker is installed separately)
	@./scripts/install-deps.sh

test: ## Run the Go unit tests with coverage threshold
	$(MAKE) -C app test

lint: ## Run golangci-lint on the Go code
	$(MAKE) -C app lint

scan: ## Build the Docker image and scan it with Trivy
	$(MAKE) -C app image-build scan

chart-lint: ## Lint the Helm chart with Helm 4
	$(MAKE) -C helm-chart lint
