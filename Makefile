.PHONY: build test run-api run-tui docker-build docker-up docker-down clean

PATH := /opt/go/bin:$(PATH)

build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/proxygateway-api ./cmd/proxygateway-api
	go build -o bin/proxygateway-tui ./cmd/proxygateway-tui

test:
	@echo "Running test suite..."
	go test -v -race ./...

run-api:
	go run ./cmd/proxygateway-api

run-tui:
	go run ./cmd/proxygateway-tui

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/ cover.out coverage.html
