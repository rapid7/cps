# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=cps
BINARY_LINUX=$(BINARY_NAME)_linux_amd64
GIT_COMMIT := $(shell git rev-parse --short HEAD)

all: test build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v -ldflags "-s -w -X github.com/rapid7/cps/version.GitCommit=$(GIT_COMMIT)"
test:
	$(GOTEST) -v ./...
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_LINUX)

run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_LINUX) -v 	-ldflags "-s -w -X github.com/rapid7/cps/version.GitCommit=$(GIT_COMMIT)"
