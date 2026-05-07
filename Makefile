# ----------------------------------------------------------------------------
# CSSI - Container Storage Snapshot-aware Interface (NFS + LVM)
# ----------------------------------------------------------------------------

SHELL := /usr/bin/env bash

# Module / packaging
MODULE      ?= github.com/michaelcourcyo/cssi
BIN_DIR     ?= bin
DIST_DIR    ?= dist

# Binaries to build (one entry per cmd/<name>)
BINARIES    := cssi-driver cssi-server

# Container images
REGISTRY    ?= ghcr.io/michaelcourcyo
IMAGE_TAG   ?= dev

# Build metadata baked into the binary
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG := $(MODULE)/internal/version
LDFLAGS     := -s -w \
               -X $(VERSION_PKG).Version=$(VERSION) \
               -X $(VERSION_PKG).Commit=$(COMMIT) \
               -X $(VERSION_PKG).Date=$(DATE)

# Cross-compilation defaults (override on the command line, e.g. GOOS=linux)
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

GO          := go
GOFLAGS     := -trimpath

.DEFAULT_GOAL := build

# ----------------------------------------------------------------------------
# Help
# ----------------------------------------------------------------------------

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	      /^[a-zA-Z0-9_.-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ----------------------------------------------------------------------------
# Build
# ----------------------------------------------------------------------------

.PHONY: build
build: $(addprefix build-,$(BINARIES)) ## Build all binaries into $(BIN_DIR)/.

# Pattern rule: `make build-cssi-driver`, `make build-cssi-server`, ...
.PHONY: $(addprefix build-,$(BINARIES))
$(addprefix build-,$(BINARIES)): build-%: $(BIN_DIR)
	@echo ">> building $* ($(GOOS)/$(GOARCH))"
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' \
			-o $(BIN_DIR)/$* ./cmd/$*

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

.PHONY: install
install: ## Install binaries into $GOBIN.
	$(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/...

# ----------------------------------------------------------------------------
# Quality
# ----------------------------------------------------------------------------

.PHONY: fmt
fmt: ## Run gofmt over the tree.
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet.
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy.
	$(GO) mod tidy

.PHONY: test
test: ## Run unit tests.
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: lint
lint: ## Run golangci-lint (must be installed).
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not installed: https://golangci-lint.run/"; exit 1; }
	golangci-lint run ./...

.PHONY: check
check: fmt vet test ## Run fmt, vet and tests.

# ----------------------------------------------------------------------------
# Container images
# ----------------------------------------------------------------------------

.PHONY: docker
docker: $(addprefix docker-,$(BINARIES)) ## Build all container images.

.PHONY: $(addprefix docker-,$(BINARIES))
$(addprefix docker-,$(BINARIES)): docker-%:
	@echo ">> building image $(REGISTRY)/$*:$(IMAGE_TAG)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		-f build/$*/Dockerfile \
		-t $(REGISTRY)/$*:$(IMAGE_TAG) \
		.

# ----------------------------------------------------------------------------
# Housekeeping
# ----------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.txt
