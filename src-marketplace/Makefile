.PHONY: run build test lint

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -o marketplace ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run
