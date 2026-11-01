.PHONY: build run test

build:
	go build -o bin/tinyupdater ./cmd/tinyupdater

run:
	go run ./cmd/tinyupdater server --config config.yaml

test:
	go test ./...
