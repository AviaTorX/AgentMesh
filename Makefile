.PHONY: help build run demo test clean docker-up docker-down deps fmt lint build-distributed run-distributed

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Install Go dependencies
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

build: build-distributed ## Build distributed agent system (default)

build-demo: deps ## Build the e-commerce demo
	@echo "Building E-Commerce Demo..."
	go build -o bin/ecommerce examples/ecommerce.go
	@echo "Build complete: bin/ecommerce"

build-distributed: deps ## Build distributed agent system (agent + managers)
	@echo "Building Distributed System..."
	go build -o bin/agent cmd/agent/main.go
	go build -o bin/topology-manager cmd/topology-manager/main.go
	go build -o bin/consensus-manager cmd/consensus-manager/main.go
	go build -o bin/knowledge-manager cmd/knowledge-manager/main.go
	go build -o bin/api-server cmd/api-server/main.go
	@echo "Build complete: bin/agent, bin/topology-manager, bin/consensus-manager, bin/knowledge-manager, bin/api-server"

docker-up: ## Start Docker infrastructure (Kafka, Redis, Prometheus)
	@echo "Starting Docker infrastructure..."
	cd deployments && docker-compose up -d
	@echo "Infrastructure running!"
	@echo "   Kafka: localhost:9092"
	@echo "   Redis: localhost:6379"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana: http://localhost:3500 (admin/admin)"

docker-down: ## Stop Docker infrastructure
	@echo "Stopping Docker infrastructure..."
	cd deployments && docker-compose down
	@echo "Infrastructure stopped"

docker-logs: ## Show Docker logs
	cd deployments && docker-compose logs -f

run: build-distributed docker-up ## Build and run distributed system
	@echo "Starting Distributed AgentMesh System..."
	@sleep 3
	./scripts/demo-unified.sh

demo: build-demo docker-up ## Build and run the e-commerce demo
	@echo "Starting E-Commerce Demo..."
	@sleep 3
	./bin/ecommerce

run-distributed: build-distributed docker-up ## Build and run distributed system
	@echo "Starting Distributed AgentMesh System..."
	@sleep 3
	./scripts/run-distributed.sh

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Coverage report:"
	go tool cover -func=coverage.out

test-coverage: test ## Generate HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@echo "Code formatted"

lint: ## Run linter
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run ./...
	@echo "Linting complete"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Cleaned"

status: ## Show infrastructure status
	@echo "Infrastructure Status:"
	@cd deployments && docker-compose ps

all: clean deps build build-demo test ## Build everything and run tests

.DEFAULT_GOAL := help
