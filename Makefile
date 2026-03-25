APP := classgo
BIN := bin/$(APP)
PID_FILE := bin/.pid

.PHONY: help tidy build build-all test start stop

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

tidy: ## Run fmt, vet, and mod tidy
	go fmt ./...
	go vet ./...
	go mod tidy

test: ## Run tests
	go test -v -count=1 ./...

build: tidy ## Build binary to bin/
	@mkdir -p bin
	go build -o $(BIN) .

build-all: tidy ## Build for Windows, macOS, and Linux
	@mkdir -p bin
	GOOS=darwin  GOARCH=amd64 go build -o bin/$(APP)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o bin/$(APP)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64 go build -o bin/$(APP)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -o bin/$(APP)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/$(APP)-windows-amd64.exe .
	@echo "Binaries in bin/"

start: build ## Start the server in the background
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(APP) is already running (PID $$(cat $(PID_FILE)))"; \
	else \
		$(BIN) & echo $$! > $(PID_FILE); \
		echo "$(APP) started (PID $$(cat $(PID_FILE)))"; \
	fi

stop: ## Stop the running server
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		kill $$(cat $(PID_FILE)); \
		rm -f $(PID_FILE); \
		echo "$(APP) stopped"; \
	else \
		echo "$(APP) is not running"; \
		rm -f $(PID_FILE); \
	fi
