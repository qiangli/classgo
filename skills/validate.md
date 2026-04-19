---
name: validate
description: Start a test server with example data and run integration tests against all ClassGo endpoints, flows, and PIN modes
user_invocable: true
args: "[url]"
---

# Validate ClassGo

Run integration tests against a ClassGo server. If no URL is provided, automatically start a dedicated test instance on port 9090 using `data/csv.example` and a temporary database — then tear it down when done.

## Startup

If a URL argument is provided, use it as `BASE` and skip startup/teardown. Otherwise, start a test server:

```bash
# Build first (Go-only, skip frontend if templates/static already exist)
go build -o bin/classgo .

# Start test instance on port 9090 with example data and temp DB
TEST_DB=$(mktemp /tmp/classgo-test-XXXXXX.db)
bin/classgo -port 9090 -data-dir data/csv.example -db "$TEST_DB" &
TEST_PID=$!
sleep 2

# Verify it started
if ! kill -0 $TEST_PID 2>/dev/null; then
  echo "FAIL: test server did not start"
  rm -f "$TEST_DB"
  exit 1
fi
```

Set `BASE=http://localhost:9090` for all tests below.

## Teardown

After all tests complete (pass or fail), clean up:

```bash
kill $TEST_PID 2>/dev/null
wait $TEST_PID 2>/dev/null
rm -f "$TEST_DB"
```

## Test Plan

Run ALL of the following tests. Use `curl` for each. Report pass/fail for each test with the HTTP status code. Stop and report on first critical failure.

**Test students from `data/csv.example`:**
- Active: Alice Wang (S001), Bob Wang (S002), Carlos Garcia (S003), Diana Chen (S004), Emma Taylor (S005), Frank Miller (S006), Grace Lee (S007), Henry Kim (S008), Ivy Patel (S009), Jack Brown (S010)
- Inactive: Karen Davis (S011), Leo Martinez (S012)

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
curl -s "$BASE/api/status?student_name=Alice+Wang"
# Expect: {"checked_in": false}

# Student search — active students only
curl -s "$BASE/api/students/search?q=a"
# Expect: JSON array containing Alice, Carlos, Diana, etc.

# Student search — inactive student should NOT appear
curl -s "$BASE/api/students/search?q=Karen"
# Expect: empty array []
```

### 3. Check-In/Check-Out Flow — PIN Mode: Off

First, ensure PIN mode is off. Then test the full flow:

```bash
# Check in an active student (no PIN needed)
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Alice Wang","device_type":"mobile"}'
# Expect: {"ok": true, "message": "Welcome, Alice Wang!"}

# Verify status shows checked in
curl -s "$BASE/api/status?student_name=Alice+Wang"
# Expect: {"checked_in": true, "checked_out": false}

# Duplicate check-in should say "already"
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Alice Wang","device_type":"mobile"}'
# Expect: message contains "Already"

# Check out
curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Alice Wang"}'
# Expect: {"ok": true, "message": "Goodbye, Alice Wang!"}

# Verify status shows checked out
curl -s "$BASE/api/status?student_name=Alice+Wang"
# Expect: {"checked_in": true, "checked_out": true}
```

### 4. Unregistered Student Rejected

```bash
# Check in with a name not in the system
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Nobody Special","device_type":"mobile"}'
# Expect: {"ok": false, "error": "Student not found..."}

# Check in with an inactive student
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Karen Davis","device_type":"mobile"}'
# Expect: {"ok": false, "error": "Student not found..."}
```

### 5. Check-In with Kiosk Device Type

```bash
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Bob Wang","device_type":"kiosk"}'
# Expect: {"ok": true}

curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Bob Wang"}'
# Expect: {"ok": true}
```

### 6. Check-In by Student ID

```bash
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_id":"S003","device_type":"mobile"}'
# Expect: {"ok": true, "message": "Welcome, Carlos Garcia!"}

curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Carlos Garcia"}'
# Expect: {"ok": true}
```

### 7. Device Fingerprint Capture

```bash
# Check in with fingerprint and device_id
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Diana Chen","device_type":"mobile","fingerprint":"test-fp-123","device_id":"test-dev-456"}'
# Expect: {"ok": true}

curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Diana Chen","fingerprint":"test-fp-123","device_id":"test-dev-456"}'
# Expect: {"ok": true}
```

### 8. Multiple Students — Attendance List

```bash
# Check in several students
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Emma Taylor","device_type":"mobile"}'
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Frank Miller","device_type":"kiosk"}'
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Grace Lee","device_type":"mobile"}'

# Verify attendee list
curl -s $BASE/api/attendees
# Expect: JSON array with all checked-in students for today
```

### 9. Export Endpoints (GET, expect file downloads)

```bash
# CSV export
curl -s -o /dev/null -w "%{http_code}" "$BASE/admin/export?from=2020-01-01&to=2099-12-31"
# Expect: 200 or 302

# XLSX export
curl -s -o /dev/null -w "%{http_code}" $BASE/admin/export/xlsx
# Expect: 200 or 302
```

### 10. Static Assets

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

### 11. Task Item API Tests

Test task item management endpoints. These require authentication — use cookie-based sessions. Start a test server (as above) and use `curl -b` / `curl -c` for cookie management.

```bash
# Login as admin to get session cookie
curl -s -c /tmp/cg-cookies -X POST $BASE/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}'
# Expect: {"ok": true, "role": "admin"}

# Create a global tracker item (admin only)
curl -s -b /tmp/cg-cookies -X POST $BASE/api/v1/tracker/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"Daily Math Quiz","priority":"high","recurrence":"daily","category":"Math"}'
# Expect: {"ok": true, "id": N}

# List global tracker items
curl -s -b /tmp/cg-cookies $BASE/api/v1/tracker/items
# Expect: JSON array with at least 1 item

# Create a personal task item (assigned to student)
curl -s -b /tmp/cg-cookies -X POST $BASE/api/tracker/student-items \
  -H 'Content-Type: application/json' \
  -d '{"student_id":"S001","name":"Homework Ch5","priority":"medium","recurrence":"none","requires_signoff":true}'
# Expect: {"ok": true, "id": N}

# Create a library item (no student — reusable template)
curl -s -b /tmp/cg-cookies -X POST $BASE/api/tracker/student-items \
  -H 'Content-Type: application/json' \
  -d '{"name":"Weekly Vocab Quiz","priority":"low","recurrence":"weekly","requires_signoff":true}'
# Expect: {"ok": true, "id": N}

# List items created by current user (My Items)
curl -s -b /tmp/cg-cookies $BASE/api/dashboard/teacher-items
# Expect: JSON array with both items (assigned + library)

# List items for a specific student
curl -s -b /tmp/cg-cookies "$BASE/api/tracker/student-items?student_id=S001"
# Expect: JSON array with the assigned item

# Get due items for a student today
curl -s -b /tmp/cg-cookies "$BASE/api/tracker/due?student_id=S001"
# Expect: JSON array with global + student-specific due items

# Get all tasks for a student (dashboard view)
curl -s -b /tmp/cg-cookies "$BASE/api/dashboard/all-tasks?student_id=S001"
# Expect: {"global_items": [...], "student_items": [...], "due_items": [...]}

# Delete global tracker item
curl -s -b /tmp/cg-cookies -X POST $BASE/api/v1/tracker/items/delete \
  -H 'Content-Type: application/json' \
  -d '{"id": 1}'
# Expect: {"ok": true}

# Delete personal task item
curl -s -b /tmp/cg-cookies -X POST $BASE/api/tracker/student-items/delete \
  -H 'Content-Type: application/json' \
  -d '{"id": 1}'
# Expect: {"ok": true}

rm -f /tmp/cg-cookies
```

### 12. Go Test Suite (comprehensive)

Run the full Go test suite:

```bash
go test -v -count=1 .
# Expect: all tests PASS (including tracker_test.go: 18 task item tests)

go test -v -count=1 ./internal/scheduling
# Expect: all scheduling tests PASS
```

## Reporting

After all tests complete, print a summary table:

```
Validation Results:
  Pages:
    Mobile (/)             PASS 200
    Kiosk (/kiosk)         PASS 200
    Login (/login)         PASS 200
    Admin (/admin)         PASS 200/302
    Dashboard (/dashboard) PASS 302
  APIs:
    Settings               PASS pin_mode present
    Status                 PASS checked_in=false
    Student Search         PASS active students returned
    Inactive Search        PASS empty array
  Check-In Flow (PIN Off):
    Check In               PASS Welcome
    Duplicate Check In     PASS Already
    Check Out              PASS Goodbye
    Status After           PASS checked_out=true
  Unregistered Student:
    Unknown Name           PASS rejected
    Inactive Student       PASS rejected
  Kiosk Flow:
    Kiosk Check In         PASS OK
    Kiosk Check Out        PASS OK
  Student ID Check-In:
    Check In by ID         PASS Welcome Carlos
    Check Out              PASS OK
  Fingerprint:
    FP Check In            PASS OK
    FP Check Out           PASS OK
  Multi-Student:
    Attendees List         PASS count correct
  Task Items:
    Global CRUD            PASS create/list/delete
    Personal Assigned      PASS create with student
    Library Item           PASS create without student
    My Items List          PASS teacher-items returns items
    Student Items          PASS filtered by student_id
    Due Items              PASS recurrence filtering
    All Tasks              PASS global + student items
    Delete Item            PASS soft-delete
  Exports:
    CSV Export             PASS 200/302
    XLSX Export            PASS 200/302
  Static:
    Logo                   PASS 200
    Favicon                PASS 200
    FingerprintJS          PASS 200
  Go Tests:
    Unit + Integration     PASS all passed (52 tests)
    Scheduling             PASS all passed

  Total: All passed
```

If any test fails, show the expected vs actual result.
