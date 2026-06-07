BINARY      := ledgerline
PKG         := ./...
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
IMAGE       ?= ledgerline:$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Sync go.mod/go.sum
	go mod tidy

.PHONY: fmt
fmt: ## Format the code
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofmt-formatted
	@test -z "$$(gofmt -s -l . | tee /dev/stderr)"

.PHONY: vet
vet: ## Run go vet
	go vet $(PKG)

.PHONY: lint
lint: ## Run golangci-lint (falls back to go vet if not installed)
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "golangci-lint not found; running go vet"; go vet $(PKG); fi

.PHONY: test
test: ## Run unit tests
	go test $(PKG)

.PHONY: cover
cover: ## Run tests with coverage report
	go test -race -covermode=atomic -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out | tail -n 1

.PHONY: build
build: ## Build the binary into ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/ledgerline

.PHONY: run
run: ## Run the service locally (in-memory store)
	go run ./cmd/ledgerline

.PHONY: docker-build
docker-build: ## Build the Docker image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

.PHONY: compose-up
compose-up: ## Start the full local stack (app + postgres + prometheus + grafana)
	docker compose up --build -d

.PHONY: compose-down
compose-down: ## Stop the local stack
	docker compose down -v

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	helm lint deploy/helm/ledgerline

.PHONY: tf-validate
tf-validate: ## Validate the Terraform configuration
	cd deploy/terraform && terraform init -backend=false && terraform validate

.PHONY: ci
ci: tidy fmt-check vet test ## Run the checks the CI pipeline runs
