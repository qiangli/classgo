---
name: validate
description: Run integration tests to validate all ClassGo endpoints, check-in flows, PIN modes, audit, and attendance
user_invocable: true
args: "[url]"
---

# Validate ClassGo

Run integration tests against a running ClassGo server to verify all pages, API endpoints, check-in/check-out flows (all PIN modes), audit logging, and attendance reporting. Defaults to `http://localhost:8080` unless a URL is provided.

## Prerequisites

Ensure the server is running. If not, start it first:
```bash
bin/classgo &
sleep 2
```

## Test Plan

Run ALL of the following tests. Use `curl` for each. Report pass/fail for each test with the HTTP status code. Stop and report on first critical failure.

Set `BASE` to the target URL (default `http://localhost:8080`).

### 1. Pages (GET, expect HTTP 200 and HTML content)

```bash
# Mobile check-in page
curl -s -o /dev/null -w "%{http_code}" $BASE/

# Kiosk check-in page
curl -s -o /dev/null -w "%{http_code}" $BASE/kiosk

# Login page
curl -s -o /dev/null -w "%{http_code}" $BASE/login

# Admin dashboard (may redirect to login — 200 or 302 both acceptable)
curl -s -o /dev/null -w "%{http_code}" $BASE/admin

# Dashboard (requires auth — 302 to login expected)
curl -s -o /dev/null -w "%{http_code}" $BASE/dashboard
```

### 2. API Endpoints (GET, expect JSON responses)

```bash
# Settings — should include pin_mode
curl -s $BASE/api/settings
# Expect: {"pin_mode":"off"|"center"|"per-student", "require_pin": true/false}

# Status (no student)
curl -s "$BASE/api/status?student_name=TestUser"
# Expect: {"checked_in": false}

# Student search
curl -s "$BASE/api/students/search?q=a"
# Expect: JSON array
```

### 3. Check-In/Check-Out Flow — PIN Mode: Off

First, ensure PIN mode is off. Then test the full flow:

```bash
# Check in a test student (no PIN needed)
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"ValidationTest","device_type":"mobile"}'
# Expect: {"ok": true, "message": "Welcome, ValidationTest!"}

# Verify status shows checked in
curl -s "$BASE/api/status?student_name=ValidationTest"
# Expect: {"checked_in": true, "checked_out": false}

# Duplicate check-in should say "already"
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"ValidationTest","device_type":"mobile"}'
# Expect: message contains "Already"

# Check out (no PIN needed)
curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"ValidationTest"}'
# Expect: {"ok": true, "message": "Goodbye, ValidationTest!"}

# Verify status shows checked out
curl -s "$BASE/api/status?student_name=ValidationTest"
# Expect: {"checked_in": true, "checked_out": true}
```

### 4. Check-In with Kiosk Device Type

```bash
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"KioskTest","device_type":"kiosk"}'
# Expect: {"ok": true}

curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"KioskTest"}'
# Expect: {"ok": true}
```

### 5. Device Fingerprint Capture

```bash
# Check in with fingerprint and device_id
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"FPTest","device_type":"mobile","fingerprint":"test-fp-123","device_id":"test-dev-456"}'
# Expect: {"ok": true}

curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"FPTest","fingerprint":"test-fp-123","device_id":"test-dev-456"}'
# Expect: {"ok": true}
```

### 6. Export Endpoints (GET, expect file downloads)

```bash
# CSV export
curl -s -o /dev/null -w "%{http_code}" "$BASE/admin/export?from=2020-01-01&to=2099-12-31"
# Expect: 200 or 302

# XLSX export
curl -s -o /dev/null -w "%{http_code}" $BASE/admin/export/xlsx
# Expect: 200 or 302
```

### 7. Static Assets

```bash
# Logo image
curl -s -o /dev/null -w "%{http_code}" $BASE/static/lern.png
# Expect: 200

# Favicon
curl -s -o /dev/null -w "%{http_code}" $BASE/static/favicon.svg
# Expect: 200

# FingerprintJS library
curl -s -o /dev/null -w "%{http_code}" $BASE/static/js/fingerprint.js
# Expect: 200
```

### 8. Go Test Suite (comprehensive)

Run the full Go test suite which includes:
- 10 original tests (basic check-in/check-out lifecycle)
- 24 integration tests covering:
  - PIN mode off/center/per-student (3 modes × mobile/kiosk)
  - Per-student PIN setup, validation, reset
  - Per-student PIN override (require_pin flag)
  - Rate limiting (mobile 2min, kiosk 30s, same-student allowed)
  - Audit record creation and buddy-punch flagging
  - Attendance dashboard (attendees, metrics, date range)
  - Full E2E multi-student flow

```bash
go test -v -count=1 .
# Expect: all 34 tests PASS

go test -v -count=1 ./internal/scheduling
# Expect: all 6 scheduling tests PASS
```

## Reporting

After all tests complete, print a summary table:

```
Validation Results:
  Pages:
    Mobile (/)             ✓ 200
    Kiosk (/kiosk)         ✓ 200
    Login (/login)         ✓ 200
    Admin (/admin)         ✓ 200/302
    Dashboard (/dashboard) ✓ 302
  APIs:
    Settings               ✓ pin_mode present
    Status                 ✓ checked_in=false
    Student Search         ✓ JSON array
  Check-In Flow (PIN Off):
    Check In               ✓ Welcome
    Duplicate Check In     ✓ Already
    Check Out              ✓ Goodbye
    Status After           ✓ checked_out=true
  Kiosk Flow:
    Kiosk Check In         ✓ OK
    Kiosk Check Out        ✓ OK
  Fingerprint:
    FP Check In            ✓ OK
    FP Check Out           ✓ OK
  Exports:
    CSV Export             ✓ 200/302
    XLSX Export            ✓ 200/302
  Static:
    Logo                   ✓ 200
    Favicon                ✓ 200
    FingerprintJS          ✓ 200
  Go Tests:
    Unit + Integration     ✓ 34/34 passed
    Scheduling             ✓ 6/6 passed

  Total: All passed
```

If any test fails, show the expected vs actual result.
