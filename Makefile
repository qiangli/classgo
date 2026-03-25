APP := classgo
BIN := bin/$(APP)
PID_FILE := bin/.pid

.PHONY: help tidy build start stop

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

tidy: ## Run fmt, vet, and mod tidy
	go fmt ./...
	go vet ./...
	go mod tidy

build: tidy ## Build binary to bin/
	@mkdir -p bin
	go build -o $(BIN) .

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
