APP := classgo
BIN := bin/$(APP)
PID_FILE := bin/.pid

.PHONY: tidy build start stop

tidy:
	go fmt ./...
	go vet ./...
	go mod tidy

build: tidy
	@mkdir -p bin
	go build -o $(BIN) .

start: build
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(APP) is already running (PID $$(cat $(PID_FILE)))"; \
	else \
		$(BIN) & echo $$! > $(PID_FILE); \
		echo "$(APP) started (PID $$(cat $(PID_FILE)))"; \
	fi

stop:
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		kill $$(cat $(PID_FILE)); \
		rm -f $(PID_FILE); \
		echo "$(APP) stopped"; \
	else \
		echo "$(APP) is not running"; \
		rm -f $(PID_FILE); \
	fi
