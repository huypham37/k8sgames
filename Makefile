.PHONY: build test

build:
	go build ./cmd/...

test:
	go test ./...
