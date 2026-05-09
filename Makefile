APP := classgo
BIN := bin/$(APP)
PID_FILE := bin/.pid
LOG_FILE := bin/classgo.log
DIST := dist

PLATFORMS := darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64

HOME_DIR := $(HOME)/.classgo

TAILWIND_VERSION := v4.2.2
TAILWIND_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]' | sed 's/darwin/macos/')
TAILWIND_ARCH := $(shell uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/')

.PHONY: help tidy sync build build-all test test-e2e test-e2e-setup test-e2e-headed \
        start stop start-test clean memos-frontend tailwind rclone rclone-all frp frp-all package

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

tidy: ## Run fmt, vet, and mod tidy
	go fmt ./...
	go vet ./...
	go mod tidy

sync: ## Sync and init/update all git submodules
	git submodule sync --recursive
	git submodule update --init --recursive

test: ## Run tests
	go test -v -count=1 ./internal/... .

./tailwindcss:
	@echo "Downloading tailwindcss $(TAILWIND_VERSION) for $(TAILWIND_OS)-$(TAILWIND_ARCH)..."
	curl -fsSL -o ./tailwindcss "https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)"
	chmod +x ./tailwindcss

tailwind: ./tailwindcss ## Build Tailwind CSS from templates
	./tailwindcss -i static/css/input.css -o static/css/tailwind.css --content 'templates/*.html' --minify

memos-frontend: ## Build Memos React frontend
	cd memos/web && pnpm install --frozen-lockfile && pnpm run release

rclone: ## Build rclone binary from submodule
	@if [ -d rclone-src ]; then \
		mkdir -p bin && cd rclone-src && go build -ldflags "-s" -trimpath -o ../bin/rclone . ; \
		echo "rclone built → bin/rclone"; \
	else \
		echo "rclone-src/ not found (run: git submodule update --init)"; \
	fi

rclone-all: ## Cross-compile rclone for all platforms
	@if [ -d rclone-src ]; then \
		mkdir -p bin && cd rclone-src && \
		GOOS=darwin  GOARCH=amd64 go build -ldflags "-s" -trimpath -o ../bin/rclone-darwin-amd64 . && \
		GOOS=darwin  GOARCH=arm64 go build -ldflags "-s" -trimpath -o ../bin/rclone-darwin-arm64 . && \
		GOOS=linux   GOARCH=amd64 go build -ldflags "-s" -trimpath -o ../bin/rclone-linux-amd64 . && \
		GOOS=linux   GOARCH=arm64 go build -ldflags "-s" -trimpath -o ../bin/rclone-linux-arm64 . && \
		GOOS=windows GOARCH=amd64 go build -ldflags "-s" -trimpath -o ../bin/rclone-windows-amd64.exe . ; \
		echo "rclone cross-compiled → bin/rclone-*"; \
	else \
		echo "rclone-src/ not found (run: git submodule update --init)"; \
	fi

frp: ## Build frpc binary from submodule
	@if [ -d frp-src ]; then \
		mkdir -p bin && cd frp-src && go build -tags noweb -ldflags "-s" -trimpath -o ../bin/frpc ./cmd/frpc ; \
		echo "frpc built → bin/frpc"; \
	else \
		echo "frp-src/ not found (run: git submodule update --init)"; \
	fi

frp-all: ## Cross-compile frpc for all platforms
	@if [ -d frp-src ]; then \
		mkdir -p bin && cd frp-src && \
		GOOS=darwin  GOARCH=amd64 go build -tags noweb -ldflags "-s" -trimpath -o ../bin/frpc-darwin-amd64 ./cmd/frpc && \
		GOOS=darwin  GOARCH=arm64 go build -tags noweb -ldflags "-s" -trimpath -o ../bin/frpc-darwin-arm64 ./cmd/frpc && \
		GOOS=linux   GOARCH=amd64 go build -tags noweb -ldflags "-s" -trimpath -o ../bin/frpc-linux-amd64 ./cmd/frpc && \
		GOOS=linux   GOARCH=arm64 go build -tags noweb -ldflags "-s" -trimpath -o ../bin/frpc-linux-arm64 ./cmd/frpc && \
		GOOS=windows GOARCH=amd64 go build -tags noweb -ldflags "-s" -trimpath -o ../bin/frpc-windows-amd64.exe ./cmd/frpc ; \
		echo "frpc cross-compiled → bin/frpc-*"; \
	else \
		echo "frp-src/ not found (run: git submodule update --init)"; \
	fi

build: tailwind memos-frontend rclone frp tidy ## Build binary to bin/
	@mkdir -p bin
	go build -o $(BIN) .

build-all: tidy rclone-all frp-all ## Cross-compile classgo + rclone + frpc for all platforms
	@mkdir -p bin
	GOOS=darwin  GOARCH=amd64 go build -o bin/$(APP)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o bin/$(APP)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64 go build -o bin/$(APP)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -o bin/$(APP)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/$(APP)-windows-amd64.exe .
	@echo "Binaries in bin/"

package: build-all ## Package release archives for all platforms
	@rm -rf $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%%-*}; \
		stage=$(DIST)/$(APP)-$$p; \
		mkdir -p $$stage; \
		if [ "$$os" = "windows" ]; then \
			cp bin/$(APP)-$$p.exe $$stage/$(APP).exe; \
			cp bin/rclone-$$p.exe $$stage/rclone.exe; \
			cp bin/frpc-$$p.exe $$stage/frpc.exe; \
		else \
			cp bin/$(APP)-$$p $$stage/$(APP); \
			cp bin/rclone-$$p $$stage/rclone; \
			cp bin/frpc-$$p $$stage/frpc; \
		fi; \
		cp config.json.example $$stage/config.json.example; \
		cp -r data/csv.example $$stage/data; \
		rm -rf $$stage/data/backups $$stage/data/memos $$stage/data/attendances; \
		echo "Packaged $$stage"; \
	done
	@cd $(DIST) && for p in $(PLATFORMS); do \
		os=$${p%%-*}; \
		if [ "$$os" = "windows" ]; then \
			(cd $(APP)-$$p && zip -qr ../$(APP)-$$p.zip .); \
		else \
			tar czf $(APP)-$$p.tar.gz -C $(APP)-$$p .; \
		fi; \
	done
	@echo "Archives in $(DIST)/"
	@ls -lh $(DIST)/*.tar.gz $(DIST)/*.zip 2>/dev/null

start: build ## Start the server (uses ~/.classgo by default)
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(APP) is already running (PID $$(cat $(PID_FILE)))"; \
	else \
		$(BIN) > $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE); \
		sleep 1; \
		cat $(LOG_FILE); \
		echo "$(APP) started (PID $$(cat $(PID_FILE))), logging to $(LOG_FILE)"; \
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

start-test: ## Build and start with csv.example test data
	@mkdir -p bin
	go build -o $(BIN) .
	@mkdir -p $(HOME_DIR)/data/csv $(HOME_DIR)/raw
	@cp -f data/csv.example/*.csv $(HOME_DIR)/data/csv/ 2>/dev/null || true
	@if [ -d raw ]; then cp -f raw/*.xls $(HOME_DIR)/raw/ 2>/dev/null || true; fi
	@echo "Test data copied to $(HOME_DIR)"
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(APP) is already running (PID $$(cat $(PID_FILE)))"; \
	else \
		$(BIN) > $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE); \
		sleep 1; \
		cat $(LOG_FILE); \
		echo "$(APP) started with test data (PID $$(cat $(PID_FILE)))"; \
	fi

test-e2e-setup: ## Install Playwright dependencies
	cd e2e && npm install && npx playwright install chromium

test-e2e: ## Run Playwright E2E tests (Go-only build)
	go build -o $(BIN) .
	cd e2e && npx playwright test

test-e2e-headed: ## Run E2E tests in headed browser
	go build -o $(BIN) .
	cd e2e && npx playwright test --headed

clean: ## Remove build artifacts
	rm -rf bin/ $(DIST)
	rm -f $(APP).db
