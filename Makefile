# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=cps
BINARY_LINUX=$(BINARY_NAME)_linux_amd64
GIT_COMMIT := $(shell git rev-parse --short HEAD)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
BUILDPATH=$(shell echo `pwd`/.build)/bin
GOLANG_CI_LINT_VERSION=1.39.0

# Copied from https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@echo
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; { \
		printf("$(_GREEN)make %-35s$(_RESET) $(_YELLOW)%s$(_RESET)\n", $$1, $$2) \
	}'

all: test build

build: ## Build CPS binary
	$(GOBUILD) -o $(BINARY_NAME) -v -ldflags "-s -w -X github.com/rapid7/cps/version.GitCommit=$(GIT_COMMIT)"

test: ## Run unit tests
	$(GOTEST) -v ./...

test-bench: ## Run unit tests (benchmark)
	$(GOTEST) -v -bench=. ./...

test-benchmem: ## Run unit tests (benchmem)
	$(GOTEST) -v -benchmem -bench=. ./...

test-cover: ## Run unit tests (with coverage)
	$(GOTEST) -mod=vendor -coverprofile=c.out ./...

lint-setup:
	@# Make sure linter is up to date
	$(eval CURRENT_VERSION := $(strip $(shell golangci-lint version 2>&1 | sed 's/[^0-9.]*\([0-9.]*\).*/\1/')))
	@if [ "$(CURRENT_VERSION)" != "$(GOLANG_CI_LINT_VERSION)" ]; then \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(BUILDPATH) v$(GOLANG_CI_LINT_VERSION) ; \
	fi

lint: lint-setup ## Run the linters
	@echo "Running lint for branch ${BRANCH}..."
	golangci-lint run --new-from-rev=master

clean: ## Clean
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_LINUX)
	rm -rf $(BUILDPATH)

run: ## Run CPS
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

# Cross compilation
build-linux: ## Build CPS Binary (linux amd64 target)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_LINUX) -v 	-ldflags "-s -w -X github.com/rapid7/cps/version.GitCommit=$(GIT_COMMIT)"
