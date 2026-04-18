# AGENTS.md

ClassGo is a lightweight, local-network attendance server for private tutoring, built as a single Go binary with embedded SQLite.

**Read [CLAUDE.md](./CLAUDE.md)** for detailed project conventions and Claude Code-specific guidance.

## Quick Commands

```bash
make build      # Build binary to bin/classgo
make build-all  # Cross-compile for darwin/linux/windows (amd64/arm64)
make test       # Run all tests (integration-style via httptest)
make tidy       # Format, vet, and tidy modules
make start      # Start server in background (PID-tracked)
make stop       # Stop running server
```

## Key Architecture Notes

- **Single-file server** (`main.go`): uses only stdlib + `modernc.org/sqlite` (pure Go, no CGO) and `go-qrcode`
- **Three interfaces**: `/` mobile sign-in, `/kiosk` tablet keypad, `/admin` tutor dashboard
- **Daily PIN**: 4-digit code required for all sign-in/out operations (regenerated daily, thread-safe)
- **Frontend**: Tailwind CSS via CDN, vanilla JavaScript — no build step
- **Testing**: `main_test.go` uses integration-style tests with in-memory SQLite via `setupTest()`
