# CLAUDE.md

This file provides guidance to AI agents working with code in this repository.

## Project

TutorOS (ClassGo) is a tutoring center management system built in Go. It provides student check-in/check-out attendance tracking, room scheduling, and a Memos-based communication frontend — all in a single binary with embedded SQLite databases. No external services required.

## Build & Dev

```bash
make build        # Build frontend + Go binary to bin/classgo
make build-all    # Cross-compile for darwin/linux/windows (amd64/arm64)
make test         # Run all tests: go test -v -count=1 ./...
make tidy         # Format, vet, and tidy Go modules
make start        # Start server in background (PID-tracked)
make stop         # Stop running server
```

Quick Go-only build (skip frontend): `go build -o bin/classgo .`

Run a single test: `go test -v -run TestCheckInMobile ./...`

The server listens on `:8080`. Configuration: CLI flag (`-name`) > env var (`APP_NAME`) > `config.json` > default ("LERN").

## Skills

The `skills/` directory contains reusable task definitions that any agent can execute:

- **`/build`** — Build the binary locally. Run `make build` for a full build (frontend + Go), or `go build -o bin/classgo .` for Go-only when frontend assets are already built.
- **`/deploy [start|stop|restart|status] [user@host]`** — Manage the server. Locally: uses PID file at `bin/.pid`. Remotely: SCP binary + assets, SSH to start/stop.
- **`/validate [url]`** — Integration tests against a running server (default `http://localhost:8080`). Tests all pages, API endpoints, full check-in/check-out flow, exports, and static assets.

## Architecture

### Package Structure

```
main.go                      # Entry point (~200 lines): config, DB init, Memos init, routes, server
internal/
  models/models.go           # Data structs: Student, Parent, Teacher, Room, Schedule, Attendance, Config
  database/
    migrate.go               # SQLite schema + migrations (attendance, students, rooms, schedules, etc.)
    queries.go               # TodayAttendees, SearchStudents
  datastore/
    reader.go                # XLSX + CSV import (excelize)
    writer.go                # XLSX + CSV export
    importer.go              # Spreadsheet -> SQLite pipeline
    watcher.go               # fsnotify file watcher for live reimport
  handlers/
    app.go                   # App struct, PIN management, settings, middleware
    api.go                   # Check-in/out, status, search, export handlers
    pages.go                 # Mobile, kiosk, admin, schedule page handlers
    schedule.go              # Schedule API (today, week, conflicts)
  scheduling/
    engine.go                # Materialize recurring schedule templates into sessions
    conflicts.go             # Room/teacher/student conflict detection
  memos/
    client.go                # Direct store wrapper for Memos memo CRUD
    sync.go                  # Sync student profiles + attendance into Memos
memos/                       # Embedded Memos source (v0.27.1, import-rewritten)
  server/                    # Echo HTTP server, mounted at /memos/
  store/                     # Memos data access layer (separate SQLite DB)
  web/                       # React/TypeScript frontend (Vite build)
  proto/                     # Protobuf definitions
```

### User-Facing Interfaces

| Route | Purpose |
|-------|---------|
| `/` | Mobile check-in (phone, typeahead student search) |
| `/kiosk` | Kiosk check-in (tablet, numeric keypad) |
| `/admin` | Admin dashboard (attendance, PIN control, QR codes, export) |
| `/schedule` | Weekly schedule calendar with conflict warnings |
| `/memos/` | Memos SPA (notes, communication, student profiles) |

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/checkin` | POST | Student check-in (name/ID + optional PIN) |
| `/api/checkout` | POST | Student check-out |
| `/api/status` | GET | Check if student is checked in/out |
| `/api/attendees` | GET | Today's attendance list |
| `/api/students/search` | GET | Typeahead student search by name/ID |
| `/api/settings` | GET | Current settings (PIN required, etc.) |
| `/api/admin/pin` | POST | Set custom PIN |
| `/api/admin/pin/toggle` | POST | Toggle PIN requirement on/off |
| `/api/v1/schedule/today` | GET | Today's materialized sessions |
| `/api/v1/schedule/week` | GET | This week's sessions |
| `/api/v1/schedule/conflicts` | GET | Schedule conflicts (next 4 weeks) |
| `/api/v1/memos/sync` | GET/POST | Trigger Memos data sync |
| `/admin/export` | GET | CSV attendance export |
| `/admin/export/xlsx` | GET | XLSX export (all data + attendance) |

### Key Design Details

- **Check-in PIN** is optional (toggled on admin page), auto-generated daily, or manually set by admin
- **Student search** provides typeahead filtering across first name, last name, and student ID
- **Spreadsheet import**: source of truth for entity data is `data/tutoros.xlsx` (single workbook) or `data/csv/*.csv` (folder). SQLite indexes are always rebuildable via `--rebuild-db`
- **File watcher** (fsnotify) auto-reimports when spreadsheet files change
- **Memos** is embedded in the binary — React SPA served at `/memos/`, separate SQLite DB at `data/memos/memos_prod.db`
- **Schedule engine** materializes recurring templates from `schedules.csv` into concrete sessions, with room/teacher/student conflict detection
- All dynamic routes use no-cache middleware
- Graceful shutdown on SIGINT/SIGTERM

## Data Storage

- `classgo.db` — ClassGo's SQLite database (attendance, entity indexes)
- `data/memos/memos_prod.db` — Memos' SQLite database (memos, users, attachments)
- `data/tutoros.xlsx` or `data/csv/` — Spreadsheet source of truth for students, parents, teachers, rooms, schedules
- `data/csv.example/` — Sample CSV files (committed to git)

## Testing

Tests in `main_test.go` are integration-style: `setupTest()` creates an isolated temp database and parses templates. Tests exercise HTTP handlers via `httptest` and cover check-in, check-out, duplicate prevention, status transitions, PIN validation, and device types. Scheduling tests in `internal/scheduling/engine_test.go` cover session materialization, effective date ranges, and conflict detection.

## Release

GitHub Actions (`.github/workflows/release.yml`) triggers on `v*` tags, cross-compiles for 5 platform targets, and creates a GitHub Release with compressed archives. Frontend must be pre-built before cross-compilation.
