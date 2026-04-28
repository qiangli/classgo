# AGENTS.md

This file provides guidance to AI coding assistants working in this repository.

## Build & Dev

```bash
make build        # Full build: Tailwind CSS + Memos frontend + rclone + frpc + Go binary
make build-all    # Cross-compile classgo + rclone + frpc for 5 platforms
make package      # build-all + create release archives in dist/
make test         # Run all tests: go test -v -count=1 ./internal/... .
make tidy         # Format, vet, tidy modules
make start        # Build and start server in background (PID tracked at bin/.pid)
make stop         # Stop running server
make test-e2e     # Run Playwright E2E tests (headless)
make test-e2e-headed  # Run E2E tests in headed browser
make test-e2e-setup   # Install Playwright deps + Chromium
make clean        # Remove bin/ and dist/
```

**Quick Go-only build** (skip frontend): `go build -o bin/classgo .`

**Run single test**: `go test -v -run TestCheckInMobile ./...`

**Build submodules independently**: `make rclone` / `make frp` (requires `git submodule update --init`)

Server listens on `:8080`. Config priority: CLI flag (`-name`) > env var (`APP_NAME`) > `config.json` > default ("LERN").

### Frontend Build Requirements

- **Tailwind CSS**: Requires `tailwindcss` CLI binary in repo root. Builds from `static/css/input.css` -> `static/css/tailwind.css` using templates in `templates/*.html`
- **Memos frontend**: React/TypeScript in `memos/web/`. Requires `pnpm`. Built with `pnpm install --frozen-lockfile && pnpm run release`

## Architecture

### Package Structure

```
main.go                      # Entry point: config, DB init, Memos init, routes
internal/
  models/models.go           # Data structs (Student, Attendance, TrackerItem, etc.)
  auth/                      # Session store for login/signup
  backup/                    # Multi-DB CSV-in-ZIP backup (classgo + memos DBs)
  database/
    migrate.go               # SQLite schema + migrations
    tracker.go               # Tracker items CRUD, signoff, bulk assign, auto-assign
  datastore/                 # XLSX/CSV import/export, fsnotify file watcher
  handlers/                  # HTTP handlers: check-in/out, admin, tracker, schedule
  scheduling/                # Recurring schedule materialization, conflict detection
  memos/                     # Memos client wrapper and sync
memos/                       # Embedded Memos v0.27.1 (Echo server, React SPA, separate SQLite DB)
```

### Routing & Middleware

Uses stdlib `net/http` mux directly (no third-party router). Three middleware tiers:
- **Public** (no auth): `/`, `/kiosk`, `/api/checkin`, `/api/checkout`, `/api/tracker/*`
- **Authenticated**: `RequireAuth(...)` — `/dashboard`, `/profile`, `/memos/`
- **Admin only**: `RequireAdmin(...)` / `RequireAdminAPI(...)` — `/admin/*`, `/api/v1/*`

All routes wrapped in `handlers.NoCache(...)`.

### Embedded Memos Integration

Memos runs as an embedded Go server in the same process with its own SQLite DB at `<DataDir>/memos/memos_prod.db`. ClassGo creates a Memos admin user (`tutoros`) at startup and proxies requests behind `RequireAuth`. A `MemosSyncer` keeps student data in sync. Static assets from the Memos React SPA are proxied separately (unauth'd) since they hardcode paths without the `/memos/` prefix.

### Key Business Logic

- **Checkout signoff enforcement**: `/api/checkout` blocks if student has pending `requires_signoff=true` tasks. Must use `/api/tracker/respond` to submit responses + checkout atomically.
- **Auto-assign**: On profile save, system auto-assigns tracker items for missing data with grade-aware filtering.
- **Spreadsheet source of truth**: `data/tutoros.xlsx` or `data/csv/*.csv`. SQLite indexes rebuildable via `--rebuild-db`. File watcher auto-reimports on changes.

### Data Model Notes

- `TrackerItem` (library items) vs `StudentTrackerItem` (per-student assignments) are separate structs.
- `Student.RequirePIN` + `Student.PersonalPIN` support per-student PIN override.
- `Attendance` stores both raw strings and parsed `time.Time`. The `ParseTimestamp` func strips the `Z` suffix from SQLite's `datetime('now','localtime')` output because the driver wrongly appends it.

### Data Storage

- `classgo.db` — ClassGo SQLite (attendance, entities)
- `data/memos/memos_prod.db` — Memos SQLite (separate DB)
- `data/tutoros.xlsx` or `data/csv/` — Spreadsheet source of truth
- `data/csv.example/` — Sample data (committed)

## Testing

Integration tests using `httptest` with isolated temp databases. `setupTest(t)` creates a temp SQLite, runs migrations, parses real templates, and seeds two students (`S001 Alice`, `S002 Bob`) with `PinMode: "center"` and PIN `"1234"`. No mocks — tests hit a real ephemeral SQLite DB.

- `main_test.go` — Core check-in/out, PIN validation
- `checkin_test.go` — PIN modes, rate limiting, audit trail
- `signup_test.go` — Signup/login, profile workflow, auto-assign
- `tracker_test.go` — Tracker CRUD, role-based access, bulk assign
- `e2e_test.go` — Full user flows (signup -> login -> checkin -> checkout -> signoff)
- `internal/scheduling/engine_test.go` — Schedule materialization, conflicts

### E2E Tests (Playwright)

Playwright tests in `e2e/` use page objects (`e2e/pages/`). Setup: `make test-e2e-setup` (installs npm deps + Chromium). The test suite auto-starts a Go server with a temp DB via `e2e/global-setup.ts`.

## Release

GitHub Actions triggers on `v*` tags. Release workflow cross-compiles Go only — does NOT build frontend. Frontend must be pre-built before tagging.

## Sub-directory Instructions

- See `frp-src/AGENTS.md` or `frp-src/CLAUDE.md` for FRP (frp-src) submodule guidance.
