.PHONY: help install build clean test proto run-server run-cli dev install-service uninstall-service service-status service-logs

# Variables
BINARY_NAME_CLI=todo
BINARY_NAME_SERVER=server
GOBIN=$(shell go env GOPATH)/bin
PROTO_DIR=proto
DATA_DIR=$(HOME)/.config/todolist

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

install: build ## Install binaries to $(GOBIN)
	@echo "Installing binaries to $(GOBIN)..."
	go install ./cmd/todo
	go install ./cmd/server
	@echo "✅ Installation complete!"
	@echo "   - todo: $(GOBIN)/$(BINARY_NAME_CLI)"
	@echo "   - server: $(GOBIN)/$(BINARY_NAME_SERVER)"

build: ## Build binaries (without installing)
	@echo "Building binaries..."
	go build -o $(BINARY_NAME_CLI) ./cmd/todo
	go build -o $(BINARY_NAME_SERVER) ./cmd/server
	@echo "✅ Build complete!"

clean: ## Remove built binaries
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME_CLI) $(BINARY_NAME_SERVER)
	@echo "✅ Clean complete!"

test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...
	@echo "✅ Tests complete!"

proto: ## Generate protobuf files
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/todoList.proto
	@echo "✅ Protobuf generation complete!"

run-server: ## Run the server (foreground)
	@echo "Starting server..."
	@go run ./cmd/server

run-cli: ## Run the CLI (usage: make run-cli ARGS="list")
	@go run ./cmd/todo $(ARGS)

dev: ## Install and restart server
	@echo "Installing and restarting server..."
	@pkill -f "server" 2>/dev/null || true
	@make install
	@echo "Starting server in background..."
	@$(GOBIN)/server > /tmp/todoserver.log 2>&1 &
	@sleep 1
	@echo "✅ Server restarted!"
	@echo "Test with: todo list"

setup-data: ## Setup data directory and copy initial data
	@echo "Setting up data directory..."
	@mkdir -p $(DATA_DIR)
	@if [ -f data/data.json ]; then \
		cp data/data.json $(DATA_DIR)/; \
		echo "✅ Data file copied to $(DATA_DIR)"; \
	else \
		echo '{"lists":{}}' > $(DATA_DIR)/data.json; \
		echo "✅ Created empty data file at $(DATA_DIR)"; \
	fi

uninstall: ## Uninstall binaries from $(GOBIN)
	@echo "Uninstalling binaries..."
	rm -f $(GOBIN)/$(BINARY_NAME_CLI)
	rm -f $(GOBIN)/$(BINARY_NAME_SERVER)
	@echo "✅ Uninstall complete!"

restart-server: ## Kill and restart the server
	@echo "Restarting server..."
	@pkill -f "server" 2>/dev/null || true
	@sleep 1
	@$(GOBIN)/server > /tmp/todoserver.log 2>&1 &
	@sleep 1
	@echo "✅ Server restarted!"

stop: ## Stop the server
	@echo "Stopping server..."
	@pkill -f "$(GOBIN)/server" 2>/dev/null || true
	@pkill -x "server" 2>/dev/null || true
	@echo "✅ Server stopped!"

logs: ## Show server logs
	@tail -f /tmp/todoserver.log

status: ## Check if server is running
	@if pgrep -f "$(GOBIN)/server" > /dev/null 2>&1; then \
		echo "✅ Server is running (PID: $$(pgrep -f '$(GOBIN)/server'))"; \
	elif pgrep -x "server" > /dev/null 2>&1; then \
		echo "✅ Server is running (PID: $$(pgrep -x 'server'))"; \
	else \
		echo "❌ Server is not running"; \
	fi

fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Format complete!"

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	golangci-lint run
	@echo "✅ Lint complete!"

install-service: install ## Install launchd service (user mode, starts on login)
	@echo "Installing launchd service..."
	@./scripts/install-service.sh

uninstall-service: ## Uninstall launchd service
	@echo "Uninstalling launchd service..."
	@./scripts/uninstall-service.sh

service-status: ## Check launchd service status
	@if launchctl list | grep -q "com.oceane.todolist-server"; then \
		echo "✅ Service is running"; \
		launchctl list | grep "com.oceane.todolist-server"; \
	else \
		echo "❌ Service is not running"; \
	fi

service-logs: ## Show service logs (requires service to be installed)
	@echo "Server logs:"
	@tail -f $(DATA_DIR)/server.log
