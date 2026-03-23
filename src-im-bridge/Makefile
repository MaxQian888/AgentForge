.PHONY: build run test clean

build:
	go build -o bin/im-bridge ./cmd/bridge

run:
	go run ./cmd/bridge

test:
	go test ./...

clean:
	rm -rf bin/
