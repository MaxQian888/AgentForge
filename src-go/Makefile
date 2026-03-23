.PHONY: build run test lint clean tidy

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X github.com/react-go-quick-starter/server/internal/version.Version=$(VERSION) \
  -X github.com/react-go-quick-starter/server/internal/version.Commit=$(COMMIT) \
  -X github.com/react-go-quick-starter/server/internal/version.BuildDate=$(BUILD_DATE)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v -count=1

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -rf bin/
