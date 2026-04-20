# AGENTS.md

Instructions for AI coding agents working with this repository.

## Project

TutorOS (ClassGo) is a tutoring center management system built in Go. It provides student check-in/check-out attendance tracking, task management with checkout signoff enforcement, student profiles, room scheduling, and a Memos-based communication frontend — all in a single binary with embedded SQLite databases. No external services required.

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

Reusable task definitions in `skills/`. Each file describes a workflow that agents can follow:

- **`skills/build.md`** — Build the binary locally. `make build` for full build (frontend + Go), or `go build -o bin/classgo .` for Go-only.
- **`skills/deploy.md`** — Manage the server lifecycle. Locally: uses Makefile targets and PID file at `bin/.pid`. Remotely: SCP binary + assets, SSH to start/stop.
- **`skills/validate.md`** — Integration tests against a running server (default `http://localhost:8080`). Tests all pages, API endpoints, full check-in/check-out flow, task management, exports, and static assets.

## Architecture

### Package Structure

```
main.go                      # Entry point: config, DB init, Memos init, routes, server
internal/
  models/models.go           # Data structs: Student, Parent, Teacher, Room, Schedule, Attendance, Config, TrackerItem
  auth/                      # Session store for login/signup authentication
  database/
    migrate.go               # SQLite schema + migrations (attendance, students, rooms, schedules, tracker, etc.)
    queries.go               # TodayAttendees, SearchStudents
    tracker.go               # Tracker items CRUD, due items, signoff, bulk assign, auto-assign
    seed.go                  # Sample data seeding for tracker items
  datastore/
    reader.go                # XLSX + CSV import (excelize)
    writer.go                # XLSX + CSV export
    importer.go              # Spreadsheet -> SQLite pipeline
    watcher.go               # fsnotify file watcher for live reimport
  handlers/
    app.go                   # App struct, PIN management, settings, middleware, signup/login
    api.go                   # Check-in/out (with signoff enforcement), status, search, export
    pages.go                 # Mobile, kiosk, admin, schedule page handlers
    profile.go               # Student/parent profile pages and API
    tracker.go               # Tracker items API: due, respond, complete, bulk assign
    dashboard.go             # Role-based dashboard API
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
| `/login` | Login/signup page for students and parents |
| `/dashboard` | Role-based dashboard (student/teacher/parent) |
| `/profile` | Student self-service profile page |
| `/admin` | Admin dashboard (attendance, PIN control, QR codes, export, task management) |
| `/schedule` | Weekly schedule calendar with conflict warnings |
| `/memos/` | Memos SPA (notes, communication, student profiles) |

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/checkin` | POST | Student check-in (name/ID + optional PIN) |
| `/api/checkout` | POST | Student check-out (blocked if pending signoff tasks) |
| `/api/status` | GET | Check if student is checked in/out |
| `/api/attendees` | GET | Today's attendance list |
| `/api/students/search` | GET | Typeahead student search by name/ID |
| `/api/settings` | GET | Current settings (PIN required, etc.) |
| `/api/login` | POST | Signup/login (action: signup, login, check) |
| `/api/admin/pin` | POST | Set custom PIN |
| `/api/admin/pin/toggle` | POST | Toggle PIN requirement on/off |
| `/api/tracker/due` | GET | Due tracker items for a student today |
| `/api/tracker/respond` | POST | Submit task responses + checkout atomically |
| `/api/tracker/complete` | POST | Mark a task complete/incomplete |
| `/api/tracker/student-items` | GET/POST | CRUD for per-student tracker items |
| `/api/v1/tracker/items` | GET/POST | CRUD for global tracker items (admin) |
| `/api/v1/tracker/responses` | GET | View tracker responses |
| `/api/v1/user/profile` | GET/POST | Student self-service profile |
| `/api/v1/student/profile` | GET/POST | Admin student profile management |
| `/api/dashboard/all-tasks` | GET | All tasks for a student (global + assigned) |
| `/api/dashboard/bulk-assign` | POST | Assign task to all students in a schedule |
| `/api/dashboard/assign-library-item` | POST | Assign library template to students |
| `/api/v1/schedule/today` | GET | Today's materialized sessions |
| `/api/v1/schedule/week` | GET | This week's sessions |
| `/api/v1/schedule/conflicts` | GET | Schedule conflicts (next 4 weeks) |
| `/api/v1/memos/sync` | GET/POST | Trigger Memos data sync |
| `/admin/export` | GET | CSV attendance export |
| `/admin/export/xlsx` | GET | XLSX export (all data + attendance) |

### Key Design Details

- **Three-phase rollout**: (1) simple check-in/check-out with signup, (2) admin tasks with checkout signoff enforcement, (3) full scheduling
- **Signup/login**: students and parents register with first name + last name + password. No IDs required for basic check-in — IDs are resolved automatically from the student database
- **Checkout signoff enforcement**: `/api/checkout` blocks if the student has pending `requires_signoff=true` tasks. Students must respond via `/api/tracker/respond` which records responses and checks out atomically
- **Tracker items**: global items (admin-defined, recurring) and per-student items (assigned by admin/teacher/parent, or self-created). Items with `requires_signoff=true` block checkout until completed
- **Auto-assign**: when a student saves their profile, system auto-assigns tracker items for missing data, with grade-aware filtering (e.g., PSAT 10 only for grade 10+)
- **Profile workflow**: student submits -> draft status -> admin reviews and finalizes
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

Tests are integration-style using `httptest` with isolated temp databases. Test files and coverage:

- **`main_test.go`** — Core check-in/check-out, PIN validation, status, duplicate prevention. Provides `setupTest()` base helper.
- **`checkin_test.go`** — PIN modes (off/center/per-student), rate limiting, audit trail, dashboard metrics. Uses `setupTestWithData()` with CSV example data.
- **`signup_test.go`** — Signup/login flow, profile access control, profile save/finalize, auto-assign tasks. Uses `setupSignupTest()` with Memos store.
- **`tracker_test.go`** — Tracker item CRUD, role-based access (teacher/parent/student), bulk assign, library items, signoff defaults. Uses `setupTrackerTest()`.
- **`e2e_test.go`** — Phase 1 E2E (signup -> login -> checkin -> checkout) and Phase 2 E2E (admin assigns task -> checkout blocked -> signoff -> checkout succeeds).
- **`internal/scheduling/engine_test.go`** — Schedule materialization, effective date ranges, conflict detection.

## Release

GitHub Actions (`.github/workflows/release.yml`) triggers on `v*` tags, cross-compiles for 5 platform targets, and creates a GitHub Release with compressed archives. Frontend must be pre-built before cross-compilation.
