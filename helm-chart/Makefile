# go-sse Helm chart Makefile (requires Helm 4).
# Works with GNU Make 3.81+ (macOS default). Run `make help` to list the targets.

HELM_VERSION    := 4.3.2
HELM_IMAGE      := alpine/helm:$(HELM_VERSION)
HELM_DOCS_IMAGE := jnorwood/helm-docs:v1.14.2
RELEASE         ?= go-sse
NAMESPACE       ?= go-sse

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

.PHONY: help check-deps install-deps check-helm helm lint template dry-run install uninstall package gen-docs sync-dashboard

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

check-deps: ## Check that all required software is installed
	@../scripts/check-deps.sh

install-deps: ## Install required software (except Docker) with apt/xcode + mise
	@../scripts/install-deps.sh

# Fails unless the local helm binary is Helm 4 (Helm 3 is not supported).
check-helm:
	@v=$$(helm version --template '{{.Version}}' 2>/dev/null || true); \
	case "$$v" in \
	  v4.*) ;; \
	  *) echo "[ERROR] Helm 4 is required (found: '$${v:-none}'). Run: make install-deps"; exit 1 ;; \
	esac

# Runs Helm 4 in a container (no local install needed): make helm CMD="version"
helm:
	docker run --rm --name helm-exec \
		--user $$(id -u):$$(id -g) \
		--mount type=bind,src="$$(pwd)",dst=/helm-chart \
		-w /helm-chart \
		-e HELM_CACHE_HOME=/helm-chart/.helm/cache \
		-e HELM_CONFIG_HOME=/helm-chart/.helm/config \
		-e HELM_DATA_HOME=/helm-chart/.helm/data \
		$(HELM_IMAGE) \
		$(CMD)

lint: ## Lint the chart (Helm 4 in Docker)
	$(MAKE) helm CMD="lint --strict ."

template: ## Render the manifests locally (Helm 4 in Docker)
	$(MAKE) helm CMD="template $(RELEASE) . --namespace $(NAMESPACE)"

package: ## Package the chart into ./packages (Helm 4 in Docker)
	$(MAKE) helm CMD="package . -d packages"

gen-docs: ## Regenerate README.md from README.md.gotmpl and values.yaml
	docker run --rm --name helm-docs \
		--user $$(id -u):$$(id -g) \
		--mount type=bind,src="$$(pwd)",dst=/helm-chart \
		-w /helm-chart \
		$(HELM_DOCS_IMAGE) \
		helm-docs

sync-dashboard: ## Copy the Grafana dashboard from ../app/dashboard
	cp ../app/dashboard/go-sse-dashboard.json dashboards/go-sse-dashboard.json

dry-run: check-helm ## Simulate the install against the current cluster
	helm upgrade --install $(RELEASE) . \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--dry-run=server \
		--debug

install: check-helm ## Install/upgrade the chart in the current cluster
	helm upgrade --install $(RELEASE) . \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--wait

uninstall: check-helm ## Uninstall the chart
	helm uninstall $(RELEASE) \
		--namespace $(NAMESPACE)
