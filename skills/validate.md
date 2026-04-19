---
name: validate
description: Run integration tests to validate all ClassGo endpoints and pages
user_invocable: true
args: "[url]"
---

# Validate ClassGo

Run integration tests against a running ClassGo server to verify all pages and API endpoints are working. Defaults to `http://localhost:8080` unless a URL is provided.

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

# Admin dashboard
curl -s -o /dev/null -w "%{http_code}" $BASE/admin

# Schedule page
curl -s -o /dev/null -w "%{http_code}" $BASE/schedule

# Memos SPA
curl -s -o /dev/null -w "%{http_code}" $BASE/memos/

# Memos health check
curl -s $BASE/memos/healthz
```

### 2. API Endpoints (GET, expect JSON responses)

```bash
# Settings
curl -s $BASE/api/settings
# Expect: {"require_pin": true/false}

# Status (no student)
curl -s "$BASE/api/status?student_name=TestUser"
# Expect: {"checked_in": false}

# Today's attendees
curl -s $BASE/api/attendees
# Expect: JSON array

# Student search
curl -s "$BASE/api/students/search?q=a"
# Expect: JSON array

# Schedule - today
curl -s $BASE/api/v1/schedule/today
# Expect: JSON array

# Schedule - week
curl -s $BASE/api/v1/schedule/week
# Expect: JSON array

# Schedule - conflicts
curl -s $BASE/api/v1/schedule/conflicts
# Expect: JSON array
```

### 3. Check-In/Check-Out Flow (POST, full lifecycle)

First, get the current PIN requirement:
```bash
SETTINGS=$(curl -s $BASE/api/settings)
```

If PIN is required, get it from the admin page or use the API to temporarily disable it.

Then test the full flow:

```bash
# Check in a test student
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"ValidationTest","pin":"","device_type":"mobile"}'
# Expect: {"ok": true, "message": "Welcome, ValidationTest!"}

# Verify status shows checked in
curl -s "$BASE/api/status?student_name=ValidationTest"
# Expect: {"checked_in": true, "checked_out": false}

# Duplicate check-in should say "already"
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"ValidationTest","pin":"","device_type":"mobile"}'
# Expect: message contains "Already"

# Check out
curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"ValidationTest","pin":""}'
# Expect: {"ok": true, "message": "Goodbye, ValidationTest!"}

# Verify status shows checked out
curl -s "$BASE/api/status?student_name=ValidationTest"
# Expect: {"checked_in": true, "checked_out": true}
```

### 4. Export Endpoints (GET, expect file downloads)

```bash
# CSV export
curl -s -o /dev/null -w "%{http_code}" $BASE/admin/export
# Expect: 200

# XLSX export
curl -s -o /dev/null -w "%{http_code}" $BASE/admin/export/xlsx
# Expect: 200
```

### 5. Static Assets

```bash
# Logo image
curl -s -o /dev/null -w "%{http_code}" $BASE/static/lern.png
# Expect: 200

# Favicon
curl -s -o /dev/null -w "%{http_code}" $BASE/static/favicon.svg
# Expect: 200
```

## Reporting

After all tests complete, print a summary table:

```
Validation Results:
  Pages:
    Mobile (/)           ✓ 200
    Kiosk (/kiosk)       ✓ 200
    Admin (/admin)       ✓ 200
    Schedule (/schedule) ✓ 200
    Memos (/memos/)      ✓ 200
    Memos Health         ✓ OK
  APIs:
    Settings             ✓ 200
    Status               ✓ 200
    Attendees            ✓ 200
    Student Search       ✓ 200
    Schedule Today       ✓ 200
    Schedule Week        ✓ 200
    Schedule Conflicts   ✓ 200
  Check-In Flow:
    Check In             ✓ OK
    Duplicate Check In   ✓ Already
    Check Out            ✓ OK
    Status After         ✓ OK
  Exports:
    CSV Export           ✓ 200
    XLSX Export          ✓ 200
  Static:
    Logo                 ✓ 200
    Favicon              ✓ 200

  Total: 20/20 passed
```

If any test fails, show the expected vs actual result.
