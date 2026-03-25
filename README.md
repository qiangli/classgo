# LERN

A lightweight, local-network attendance server for private tutoring. Runs on the tutor's laptop and lets students sign in via a shared tablet or their own phones over local Wi-Fi.

## Features

- **Mobile Sign-In** — Students scan a QR code and sign in on their phones with a "Remember Me" option
- **Kiosk Mode** — Tablet-optimized keypad interface with auto-reset for shared devices
- **Daily PIN** — Ensures students are physically present in the room
- **Admin Dashboard** — Real-time attendee list with QR code display
- **CSV Export** — Download attendance records for billing and tracking
- **Local-First** — Works without internet (after initial page load)
- **Configurable** — App name settable via config file, environment variable, or CLI flag

## Quick Start

```bash
# Build
make build

# Start the server
make start

# Stop the server
make stop
```

Open the admin dashboard at `http://localhost:8080/admin` to see the daily PIN and QR code.

## Configuration

The app name defaults to **LERN** and can be overridden (in priority order):

1. **CLI flag:** `./bin/classgo -name "MySchool"`
2. **Environment variable:** `APP_NAME=MySchool ./bin/classgo`
3. **Config file:** Create `config.json`:
   ```json
   {
     "app_name": "MySchool"
   }
   ```

## Routes

| Route | Description |
|---|---|
| `GET /` | Mobile sign-in page |
| `GET /kiosk` | Tablet kiosk sign-in |
| `GET /admin` | Tutor dashboard |
| `GET /admin/export` | CSV download |
| `POST /api/signin` | Sign-in endpoint |
| `GET /api/attendees` | Today's attendees (JSON) |

## Makefile Targets

```
make help     # Show all targets
make tidy     # Run fmt, vet, and mod tidy
make build    # Build binary to bin/
make start    # Start the server in the background
make stop     # Stop the running server
```

## Tech Stack

- **Go** — `net/http` + `html/template`
- **SQLite** — via `modernc.org/sqlite` (pure Go, no CGO)
- **QR Code** — `github.com/skip2/go-qrcode`
- **Frontend** — Tailwind CSS (CDN), vanilla JavaScript
