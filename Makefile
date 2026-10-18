# Makefile

BINARY = treeball
SRC_DIR = ./cmd/treeball

A2X = a2x
A2X_FLAGS = -a version=$(VERSION)
DOCS_DIR = ./docs
MAN_DIR = $(DOCS_DIR)/man
MAN_ADOC = $(MAN_DIR)/treeball.adoc
COMPLETIONS_DIR = $(DOCS_DIR)/completions

VERSION := $(shell \
  tag=$$(git describe --tags --exact-match 2>/dev/null); \
  if [ -n "$$tag" ]; then echo $$tag | sed 's/^v//'; \
  else git rev-parse --short=7 HEAD; fi)

.PHONY: all $(BINARY) benchmark check clean debug docs docs-clean docs-man docs-pdf docs-text docs-completions help info lint test test-coverage vendor

all: vendor $(BINARY) ## Runs the entire build chain for the application

$(BINARY): ## Builds the application
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go build -ldflags="-w -s -X main.Version=$(VERSION) -buildid=" -trimpath -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

benchmark: ## Runs the benchmark script
	@$(MAKE) $(BINARY)
	@/usr/bin/env bash ./benchmark.sh

check: ## Runs all static analysis and tests on the application code
	@$(MAKE) lint
	@$(MAKE) test

clean: ## Returns the application build stage to its original state (deleting files)
	@rm -vf $(BINARY) || true

debug: ## Builds the application in debug mode (with symbols, race checks, ...)
	CGO_ENABLED=1 GOFLAGS="-mod=vendor" go build -ldflags="-X main.Version=$(VERSION)-DBG" -trimpath -race -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

docs: ## Builds all documentation (manpages, PDF, plain text)
	@$(MAKE) docs-man
	@$(MAKE) docs-pdf
	@$(MAKE) docs-text
	@$(MAKE) docs-completions

docs-clean: ## Removes generated documentation files
	@rm -vf $(MAN_DIR)/*.pdf $(MAN_DIR)/*.text $(MAN_DIR)/*.1 $(MAN_DIR)/*.8 $(MAN_DIR)/*.xml || true
	@rm -vf $(COMPLETIONS_DIR)/* || true

docs-completions: ## Generates shell completion scripts
	@mkdir -p $(COMPLETIONS_DIR)
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) completion bash > $(COMPLETIONS_DIR)/treeball.bash
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) completion zsh > $(COMPLETIONS_DIR)/treeball.zsh
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) completion fish > $(COMPLETIONS_DIR)/treeball.fish

docs-man: ## Builds manpage documentation
	$(A2X) $(A2X_FLAGS) -f manpage $(MAN_ADOC)

docs-pdf: ## Builds PDF documentation
	$(A2X) $(A2X_FLAGS) -f pdf $(MAN_ADOC)

docs-text: ## Builds plain text documentation
	$(A2X) $(A2X_FLAGS) -f text $(MAN_ADOC)

help: ## Shows all build related commands of the Makefile
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

info: ## Shows information about the application binaries that were built
	@ldd $(BINARY) || true
	@file $(BINARY)

lint: ## Runs the linter on the application code
	@golangci-lint cache clean
	@golangci-lint run

test: ## Runs all written tests for and on the application code
	@go test -failfast -race -covermode=atomic ./...

test-coverage: ## Runs all coverage tests for and on the application code
	@go test -failfast -race -covermode=atomic -coverpkg=./... -coverprofile=coverage.tmp ./... && \
	grep -v "mock_" coverage.tmp > coverage.txt && \
	rm coverage.tmp

vendor: ## Pulls the (remote) dependencies into the local vendor folder
	@go mod tidy
	@go mod vendor
