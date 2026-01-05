.PHONY: build test test-cover clean run docker-build docker-up docker-down lint fmt help

# Binary name
BINARY=tickhook

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt

# Build flags
LDFLAGS=-ldflags="-w -s"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'

## build: Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY) ./cmd/tickhook

## test: Run all tests
test:
	$(GOTEST) -v ./...

## test-cover: Run tests with coverage report
test-cover:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-race: Run tests with race detector
test-race:
	$(GOTEST) -v -race ./...

## clean: Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY)
	rm -f coverage.out coverage.html

## run: Run with default settings (requires Redis on localhost:6379)
run: build
	./$(BINARY) --redis-url redis://localhost:6379 --auth-token dev-token --log-level debug

## lint: Run go vet
lint:
	$(GOVET) ./...

## fmt: Format code
fmt:
	$(GOFMT) ./...

## docker-build: Build Docker image
docker-build:
	docker build -t tickhook:latest .

## docker-up: Start services with docker-compose
docker-up:
	docker-compose up -d

## docker-down: Stop services
docker-down:
	docker-compose down

## docker-logs: Show logs
docker-logs:
	docker-compose logs -f tickhook

## all: Format, lint, test, and build
all: fmt lint test build
