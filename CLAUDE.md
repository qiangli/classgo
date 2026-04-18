# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

ClassGo is a local-network attendance server for private tutoring, built in Go. Students sign in via mobile phone or shared tablet (kiosk), and tutors monitor attendance on an admin dashboard. Runs as a single binary with an embedded SQLite database—no external services required.

## Build & Dev Commands

```bash
make build        # Build binary to bin/classgo
make build-all    # Cross-compile for darwin/linux/windows (amd64/arm64)
make test         # Run all tests: go test -v -count=1 ./...
make tidy         # Format, vet, and tidy modules
make start        # Start server in background (PID-tracked)
make stop         # Stop running server
```

Run a single test: `go test -v -run TestSignOutThenReSignIn ./...`

The server listens on `:8080`. Configuration priority: CLI flag (`-name`) > env var (`APP_NAME`) > `config.json` > default ("LERN").

## Architecture

**Single-file Go server** (`main.go`) using only the standard library (`net/http`, `html/template`, `database/sql`) plus two dependencies:
- `modernc.org/sqlite` — pure-Go SQLite driver (no CGO, enables easy cross-compilation)
- `github.com/skip2/go-qrcode` — QR code PNG generation

**Three user-facing interfaces**, each a Go HTML template rendered server-side:
- `/` — Mobile sign-in (phone-sized, QR code entry point)
- `/kiosk` — Kiosk sign-in (tablet-sized numeric keypad)
- `/admin` — Tutor dashboard (real-time attendance table, QR codes, CSV export)

**API endpoints:** `POST /api/signin`, `POST /api/signout`, `GET /api/status`, `GET /api/attendees`, `GET /admin/export`

**Key design details:**
- A random 4-digit PIN is generated once per calendar day and required for all sign-in/sign-out operations (thread-safe via `sync.Mutex`)
- The server auto-detects local IP and mDNS hostname to generate QR codes for students
- Frontend uses Tailwind CSS via CDN and vanilla JavaScript—no build step
- SQLite schema has a single `attendance` table with a `date(sign_in_time)` index for daily filtering
- All dynamic routes use no-cache middleware
- Graceful shutdown on SIGINT/SIGTERM

## Testing

Tests in `main_test.go` are integration-style: each test calls `setupTest()` which creates an isolated in-memory database and parses templates. Tests exercise HTTP handlers via `httptest` and cover sign-in, sign-out, duplicate prevention, status transitions, and validation.

## Release

GitHub Actions (`.github/workflows/release.yml`) triggers on `v*` tags, cross-compiles for 5 platform targets, and creates a GitHub Release with compressed archives.
