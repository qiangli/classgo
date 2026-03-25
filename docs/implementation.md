# ClassGo Implementation Plan

## Context

ClassGo is a lightweight, local-network attendance server for private tutors. The repo is fresh — only `docs/plan.md` exists. We need to build the entire application from scratch: Go backend with SQLite, three HTML templates, and supporting infrastructure.

## Dependencies

- `modernc.org/sqlite` — pure-Go SQLite driver (no CGO, easy cross-compilation)
- `github.com/skip2/go-qrcode` — QR code PNG generation

## Implementation Steps

### Step 1: Initialize Go module
- `go mod init classgo`
- `go get modernc.org/sqlite github.com/skip2/go-qrcode`

### Step 2: Write `main.go` (all backend logic)

**Data structures:**
```go
type Attendance struct {
    ID, StudentID, StudentName, DeviceType string
    Timestamp time.Time
}
```

**Global state:** `db *sql.DB`, `dailyPIN string`, `pinDate string`, `mu sync.Mutex`

**Key functions:**
| Function | Purpose |
|---|---|
| `initDB()` | Open SQLite, create `attendance` table with index |
| `ensureDailyPIN() string` | Rotate 4-digit PIN daily (in-memory, lazy on first request of the day) |
| `getLocalIP() string` | Scan `net.Interfaces()` for first non-loopback IPv4 |
| `generateQR(url) string` | Return base64 data URI of QR PNG |
| `handleMobile(w, r)` | Render mobile.html |
| `handleKiosk(w, r)` | Render kiosk.html |
| `handleAdmin(w, r)` | Query today's attendance, gen QR, render admin.html |
| `handleSignIn(w, r)` | POST: validate PIN, check duplicate, insert, return JSON |
| `handleAttendees(w, r)` | GET: return today's attendees as JSON (for admin polling) |
| `handleExport(w, r)` | GET: CSV download with optional `?from=&to=` date filter |

**Database schema:**
```sql
CREATE TABLE IF NOT EXISTS attendance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id TEXT NOT NULL,
    student_name TEXT NOT NULL,
    device_type TEXT NOT NULL CHECK(device_type IN ('mobile','kiosk')),
    timestamp DATETIME DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(date(timestamp));
```

**Daily PIN:** Generated as `fmt.Sprintf("%04d", rand.Intn(10000))`. Displayed only on admin page. Validated on sign-in. Regenerated on new calendar day or server restart.

**Duplicate prevention:** Check if `student_id` already signed in today before inserting.

**Server:** Bind to `0.0.0.0:8080`, graceful shutdown on SIGINT/SIGTERM.

### Step 3: Write `templates/admin.html`
- Large PIN display for tutor to announce
- QR code image (data URI from server)
- Server URL text for manual entry
- Attendee table with count, auto-refreshed via JS polling `/api/attendees` every 5s
- "Export CSV" link to `/admin/export`
- Tailwind CSS via CDN

### Step 4: Write `templates/mobile.html`
- Form: Student ID, Student Name, PIN input
- "Remember Me" checkbox — saves to `localStorage`, pre-fills on return
- Submit via `fetch('/api/signin')`, show success/error inline
- Tailwind CSS via CDN

### Step 5: Write `templates/kiosk.html`
- Large on-screen numeric keypad
- Two-step flow: Student ID → PIN
- On success: full-screen green checkmark with student name for 3 seconds, then auto-reset
- No localStorage (shared device)
- Tailwind CSS via CDN

### Step 6: Add `.gitignore`
- `classgo.db`, IDE files, binary output

## Files to Create/Modify
- `go.mod` / `go.sum` — new
- `main.go` — new (all Go logic)
- `templates/admin.html` — new
- `templates/mobile.html` — new
- `templates/kiosk.html` — new
- `.gitignore` — new

## Verification
1. `go build` — compiles without errors
2. `go run main.go` — starts server, prints local IP and PIN
3. Open `http://localhost:8080/admin` — see PIN, QR code, empty attendee list
4. Scan QR or open `http://localhost:8080/` on phone — see mobile sign-in form
5. Sign in with correct PIN — admin page updates within 5s
6. Sign in again with same student ID — get "already signed in" message
7. Open `http://localhost:8080/kiosk` — verify keypad UI and auto-reset flow
8. Click "Export CSV" on admin — download CSV with attendance records
