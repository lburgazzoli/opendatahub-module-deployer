.DEFAULT_GOAL := help

GOLANGCI_LINT_VERSION ?= v2.14.0
GOVULNCHECK_VERSION ?= v1.8.0

GOLANGCI_LINT = go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOVULNCHECK = go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

##@ General
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9\/]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)}' $(MAKEFILE_LIST)

##@ Dependencies
.PHONY: deps
deps: ## Tidy go.mod and go.sum
	go mod tidy

##@ Formatting
.PHONY: fmt
fmt: ## Format source code
	go fmt ./...

##@ Testing
.PHONY: test
test: ## Run all tests with the race detector
	go test -race -count=1 ./...

.PHONY: test/unit
test/unit: ## Run unit tests with the race detector
	go test -short -race -count=1 ./...

.PHONY: test/e2e
test/e2e: ## Run e2e-tagged tests with the race detector
	go test -race -count=1 -tags e2e ./...

##@ Linting
.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: lint/fix
lint/fix: ## Run golangci-lint with auto-fix
	$(GOLANGCI_LINT) run --fix ./...

##@ Security
.PHONY: vulncheck
vulncheck: ## Run govulncheck
	$(GOVULNCHECK) ./...
