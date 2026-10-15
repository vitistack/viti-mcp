BINARY_NAME ?= viti-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

CONTAINER_TOOL ?= docker

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= latest
GOSEC ?= $(LOCALBIN)/gosec
# Pinned to gosec main: v2.29.0 bundles golang.org/x/tools v0.49.0, which cannot read Go 1.27.2 export data. Switch back to latest once v2.29.1+ is released.
GOSEC_VERSION ?= master
GOVULNCHECK ?= $(LOCALBIN)/govulncheck
GOVULNCHECK_VERSION ?= latest

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: fix
fix: ## Run go fix against code.
	go fix ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile coverage.out

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

##@ Security

.PHONY: gosec
gosec: install-security-scanner ## Run gosec security scan.
	$(GOSEC) ./...

.PHONY: govulncheck
govulncheck: install-govulncheck ## Run govulncheck vulnerability scan.
	$(GOVULNCHECK) ./...

##@ Dependencies

deps: ## Download and verify dependencies.
	@go mod download
	@go mod verify
	@go mod tidy

update-deps: ## Update dependencies.
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "Dependencies updated!"

##@ Build

.PHONY: build
build: fmt vet ## Build the viti-mcp binary.
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/viti-mcp

.PHONY: run
run: fmt vet ## Run the MCP server locally.
	go run $(LDFLAGS) ./cmd/viti-mcp $(ARGS)

.PHONY: install
install: build ## Install the binary to GOBIN.
	cp bin/$(BINARY_NAME) $(shell go env GOBIN 2>/dev/null || echo "$(shell go env GOPATH)/bin")/$(BINARY_NAME)

.PHONY: docker-build
docker-build: ## Build docker image.
	$(CONTAINER_TOOL) build -t $(BINARY_NAME):$(VERSION) .

##@ Tools

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: install-security-scanner
install-security-scanner: $(GOSEC)
$(GOSEC): $(LOCALBIN)
	@set -e; echo "Installing gosec $(GOSEC_VERSION)"; \
	GOBIN=$(LOCALBIN) go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)

.PHONY: install-govulncheck
install-govulncheck: $(GOVULNCHECK)
$(GOVULNCHECK): $(LOCALBIN)
	@set -e; echo "Installing govulncheck $(GOVULNCHECK_VERSION)"; \
	GOBIN=$(LOCALBIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

##@ SBOM (Software Bill of Materials)
SYFT ?= $(LOCALBIN)/syft
SYFT_VERSION ?= latest
SBOM_OUTPUT_DIR ?= sbom
SBOM_PROJECT_NAME ?= viti-mcp
SBOM_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")

.PHONY: install-syft
install-syft: $(SYFT) ## Install syft SBOM generator locally
$(SYFT): $(LOCALBIN)
	@set -e; echo "Installing syft $(SYFT_VERSION)"; \
	curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b $(LOCALBIN)

.PHONY: sbom-source
sbom-source: install-syft ## Generate SBOMs for Go source code (CycloneDX + SPDX)
	@mkdir -p $(SBOM_OUTPUT_DIR)
	@echo "Downloading Go modules for license detection..."
	go mod download
	@echo "Generating source code SBOMs..."
	$(SYFT) dir:. --source-name=$(SBOM_PROJECT_NAME) --source-version=$(SBOM_VERSION) -o cyclonedx-json=$(SBOM_OUTPUT_DIR)/sbom-source.cdx.json
	$(SYFT) dir:. --source-name=$(SBOM_PROJECT_NAME) --source-version=$(SBOM_VERSION) -o spdx-json=$(SBOM_OUTPUT_DIR)/sbom-source.spdx.json
	@echo "SBOMs generated: $(SBOM_OUTPUT_DIR)/sbom-source.{cdx,spdx}.json"

.PHONY: sbom-container
sbom-container: install-syft ## Generate SBOMs for container image (CycloneDX + SPDX, requires IMG)
	@mkdir -p $(SBOM_OUTPUT_DIR)
	@echo "Generating container SBOMs for $(IMG)..."
	$(SYFT) $(IMG) -o cyclonedx-json=$(SBOM_OUTPUT_DIR)/sbom-container.cdx.json
	$(SYFT) $(IMG) -o spdx-json=$(SBOM_OUTPUT_DIR)/sbom-container.spdx.json
	@echo "SBOMs generated: $(SBOM_OUTPUT_DIR)/sbom-container.{cdx,spdx}.json"

.PHONY: sbom
sbom: sbom-source ## Alias for sbom-source

.PHONY: sbom-all
sbom-all: sbom-source sbom-container ## Generate all SBOMs (source + container)

define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

##@ Pre-commit

# Make targets precheck runs, in order (mirrors what CI runs on push).
PRECHECK_TARGETS ?= build lint test gosec govulncheck
# Helm charts precheck lints (skipped when helm is not installed).
PRECHECK_HELM_CHARTS ?= $(wildcard charts/*/)

# Snapshot of the working tree (tracked diff + untracked files) used to detect
# files rewritten by go mod tidy / code generation / go fmt during precheck.
PRECHECK_TREE_STATE = { git diff HEAD; git ls-files --others --exclude-standard; } | git hash-object --stdin

.PHONY: precheck
precheck: ## Run every check CI runs (tidy, build, lint, test, security scans, helm lint) before commit/push.
	@set -e; \
	before=$$($(PRECHECK_TREE_STATE)); \
	if [ -f go.mod ]; then \
		printf '%b\n' "\033[0;36m==> go mod tidy + verify\033[0m"; \
		go mod tidy; \
		go mod verify; \
	fi; \
	for t in $(PRECHECK_TARGETS); do \
		printf '%b\n' "\033[0;36m==> make $$t\033[0m"; \
		$(MAKE) --no-print-directory $$t; \
	done; \
	if [ -n "$(strip $(PRECHECK_HELM_CHARTS))" ]; then \
		if command -v helm >/dev/null 2>&1; then \
			for c in $(PRECHECK_HELM_CHARTS); do \
				printf '%b\n' "\033[0;36m==> helm lint $$c\033[0m"; \
				helm lint $$c; \
			done; \
		else \
			printf '%b\n' "\033[0;33mhelm not installed, skipping helm lint\033[0m"; \
		fi; \
	fi; \
	after=$$($(PRECHECK_TREE_STATE)); \
	if [ "$$before" != "$$after" ]; then \
		printf '%b\n' "\033[0;33m\033[1mprecheck modified files (go.mod/go.sum, generated code, CRDs/RBAC or formatting).\033[0m"; \
		printf '%b\n' "\033[0;33mReview and stage them before committing:\033[0m"; \
		git status --short; \
		exit 1; \
	fi; \
	printf '%b\n' "\033[0;32m\033[1mAll prechecks passed - ready to commit and push.\033[0m"
