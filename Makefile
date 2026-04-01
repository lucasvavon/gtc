VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  ?= gtc
PKG     ?= ./...
LDFLAGS ?= -ldflags "-X github.com/lucasvavon/gtc/cmd.Version=$(VERSION)"

.PHONY: build install tidy fmt lint test check

build:
	go build $(LDFLAGS) -o $(BINARY) .

install:
	go install $(LDFLAGS) .

tidy:
	go mod tidy

fmt:
	go fmt $(PKG)

lint:
	golangci-lint run $(PKG)

test:
	go test $(PKG) -v

check: fmt lint test