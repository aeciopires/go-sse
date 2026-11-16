#---------------------------
# go-sse application Makefile
# Works with GNU Make 3.81+ (macOS default) and bash 3.2+.
# Run `make help` to list the targets.
#---------------------------

# Change the value according to new releases
VERSION := 2.0.0

APP_NAME        := go-sse
DOCKER_REPO     ?= aeciopires
IMAGE           := $(DOCKER_REPO)/$(APP_NAME)
BIN_FILE        := bin/$(APP_NAME)
COVERAGE_FILE   := coverage.out
COVERAGE_MIN    := 90
TRIVY_VERSION   := 0.76.2
TRIVY_SEVERITY  ?= HIGH,CRITICAL
LDFLAGS         := -s -w -X main.version=$(VERSION)

# Platforms supported. Reference: https://docs.docker.com/build/building/multi-platform/
SUPPORTED_PLATFORMS := linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6

# /bin/bash exists on both Ubuntu and macOS.
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

.PHONY: help check-deps install-deps deps update-deps fmt vet lint test coverage-html build run clean \
        image-build scan image up down logs recreate compose-up compose-down compose-logs dashboard

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

#--------------------------- Dependencies

check-deps: ## Check that all required software is installed
	@../scripts/check-deps.sh

install-deps: ## Install required software (except Docker) with apt/xcode + mise
	@../scripts/install-deps.sh

deps: ## Download Go module dependencies
	go mod download

update-deps: ## Update Go dependencies to their latest versions
	go get -u ./...
	go mod tidy

#--------------------------- Quality

fmt: ## Format Go code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (installed by mise)
	golangci-lint run ./...

test: ## Run unit tests with race detector and enforce coverage >= COVERAGE_MIN%
	go test -race -count=1 -covermode=atomic -coverpkg=./... -coverprofile=$(COVERAGE_FILE) ./...
	@total=$$(go tool cover -func=$(COVERAGE_FILE) | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	echo "Total coverage: $${total}% (minimum: $(COVERAGE_MIN)%)"; \
	awk -v t="$$total" -v m="$(COVERAGE_MIN)" 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' \
	  || { echo "[ERROR] coverage below $(COVERAGE_MIN)%"; exit 1; }

coverage-html: test ## Open the HTML coverage report
	go tool cover -html=$(COVERAGE_FILE)

#--------------------------- Build and run locally

build: ## Build the binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_FILE) .

run: build ## Build and run the binary (LOG_LEVEL=DEBUG make run)
	./$(BIN_FILE)

clean: ## Remove build and coverage artifacts
	rm -f $(BIN_FILE) $(COVERAGE_FILE)

#--------------------------- Docker image

image-build: ## Build the Docker image for the local platform
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

scan: ## Scan the Docker image for vulnerabilities with Trivy (fails on HIGH/CRITICAL)
	@if command -v trivy >/dev/null 2>&1; then \
	  trivy image --severity $(TRIVY_SEVERITY) --ignore-unfixed --exit-code 1 $(IMAGE):$(VERSION); \
	else \
	  echo "trivy not found locally, using the aquasec/trivy:$(TRIVY_VERSION) container"; \
	  mkdir -p "$${HOME}/.cache/trivy"; \
	  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v "$${HOME}/.cache/trivy:/root/.cache/trivy" \
	    aquasec/trivy:$(TRIVY_VERSION) image --severity $(TRIVY_SEVERITY) --ignore-unfixed --exit-code 1 $(IMAGE):$(VERSION); \
	fi

image: ## Build multi-platform images, push to Docker Hub and scan with Trivy
	@read -rp 'Username of Docker Hub: ' DOCKER_HUB_ACCOUNT; \
	docker login -u "$${DOCKER_HUB_ACCOUNT}"; \
	docker buildx inspect multi-platform-builder >/dev/null 2>&1 \
	  || docker buildx create --platform="$(SUPPORTED_PLATFORMS)" --name multi-platform-builder; \
	docker buildx build --builder multi-platform-builder --push --platform="$(SUPPORTED_PLATFORMS)" \
	  --build-arg VERSION=$(VERSION) \
	  -t "$${DOCKER_HUB_ACCOUNT}/$(APP_NAME):$(VERSION)" \
	  -t "$${DOCKER_HUB_ACCOUNT}/$(APP_NAME):latest" .; \
	$(MAKE) scan DOCKER_REPO="$${DOCKER_HUB_ACCOUNT}"

#--------------------------- Single container

up: ## Run the image in a single container (no observability stack)
	docker run -d -p 8080:8080 --rm --name $(APP_NAME) $(IMAGE):$(VERSION)
	$(MAKE) logs

down: ## Stop the single container
	docker stop $(APP_NAME)

logs: ## Follow the single container logs
	docker logs -f $(APP_NAME)

recreate: ## Recreate the single container
	-$(MAKE) down
	$(MAKE) up

#--------------------------- Observability stack

compose-up: ## Start go-sse + Prometheus, Loki, Tempo, Jaeger, OTel Collector, Grafana
	$(COMPOSE) up -d --build

compose-down: ## Stop the observability stack (add -v to remove data: make compose-down ARGS=-v)
	$(COMPOSE) down $(ARGS)

compose-logs: ## Follow logs of the go-sse service
	$(COMPOSE) logs -f go-sse

dashboard: ## Regenerate dashboard/go-sse-dashboard.json and copy it to the Helm chart
	python3 dashboard/generate.py
	cp dashboard/go-sse-dashboard.json ../helm-chart/dashboards/go-sse-dashboard.json
