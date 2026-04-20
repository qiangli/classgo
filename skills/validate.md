# Validate

Run integration tests against a ClassGo server. If no URL is provided, start a dedicated test instance on port 9090 with example data and a temporary database, then tear it down when done.

## Startup

If a URL argument is provided, use it as `BASE` and skip startup/teardown. Otherwise:

```bash
go build -o bin/classgo .

TEST_DB=$(mktemp /tmp/classgo-test-XXXXXX.db)
bin/classgo -port 9090 -data-dir data/csv.example -db "$TEST_DB" &
TEST_PID=$!
sleep 2

if ! kill -0 $TEST_PID 2>/dev/null; then
  echo "FAIL: test server did not start"
  rm -f "$TEST_DB"
  exit 1
fi
```

Set `BASE=http://localhost:9090` for all tests below.

## Teardown

After all tests complete (pass or fail):

```bash
kill $TEST_PID 2>/dev/null
wait $TEST_PID 2>/dev/null
rm -f "$TEST_DB"
```

## Test Plan

Run ALL of the following tests using `curl`. Report pass/fail for each with the HTTP status code.

**Test students from `data/csv.example`:**
- Active: Alice Wang (S001), Bob Wang (S002), Carlos Garcia (S003), Diana Chen (S004), Emma Taylor (S005), Frank Miller (S006), Grace Lee (S007), Henry Kim (S008), Ivy Patel (S009), Jack Brown (S010)
- Inactive: Karen Davis (S011), Leo Martinez (S012)

### 1. Pages (GET, expect HTTP 200 and HTML)

```bash
curl -s -o /dev/null -w "%{http_code}" $BASE/           # Mobile
curl -s -o /dev/null -w "%{http_code}" $BASE/kiosk       # Kiosk
curl -s -o /dev/null -w "%{http_code}" $BASE/login       # Login
curl -s -o /dev/null -w "%{http_code}" $BASE/admin       # Admin (200 or 302)
curl -s -o /dev/null -w "%{http_code}" $BASE/dashboard   # Dashboard (302 to login)
```

### 2. API Endpoints (GET, expect JSON)

```bash
curl -s $BASE/api/settings
# Expect: {"pin_mode":"off"|"center"|"per-student", "require_pin": true/false}

curl -s "$BASE/api/status?student_name=Alice+Wang"
# Expect: {"checked_in": false}

curl -s "$BASE/api/students/search?q=a"
# Expect: JSON array with active students

curl -s "$BASE/api/students/search?q=Karen"
# Expect: empty array [] (inactive student)
```

### 3. Check-In/Check-Out Flow (PIN Off)

```bash
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Alice Wang","device_type":"mobile"}'
# Expect: {"ok": true, "message": "Welcome, Alice Wang!"}

curl -s "$BASE/api/status?student_name=Alice+Wang"
# Expect: {"checked_in": true, "checked_out": false}

curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Alice Wang","device_type":"mobile"}'
# Expect: message contains "Already"

curl -s -X POST $BASE/api/checkout \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Alice Wang"}'
# Expect: {"ok": true, "message": "Goodbye, Alice Wang!"}

curl -s "$BASE/api/status?student_name=Alice+Wang"
# Expect: {"checked_in": true, "checked_out": true}
```

### 4. Unregistered Student Rejected

```bash
curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Nobody Special","device_type":"mobile"}'
# Expect: {"ok": false, "error": "Student not found..."}

curl -s -X POST $BASE/api/checkin \
  -H 'Content-Type: application/json' \
  -d '{"student_name":"Karen Davis","device_type":"mobile"}'
# Expect: {"ok": false} (inactive student)
```

### 5. Kiosk Device Type

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
curl -s -X POST $BASE/api/checkin -H 'Content-Type: application/json' \
  -d '{"student_name":"Emma Taylor","device_type":"mobile"}'
curl -s -X POST $BASE/api/checkin -H 'Content-Type: application/json' \
  -d '{"student_name":"Frank Miller","device_type":"kiosk"}'
curl -s -X POST $BASE/api/checkin -H 'Content-Type: application/json' \
  -d '{"student_name":"Grace Lee","device_type":"mobile"}'

curl -s $BASE/api/attendees
# Expect: JSON array with all checked-in students for today
```

### 9. Export Endpoints

```bash
curl -s -o /dev/null -w "%{http_code}" "$BASE/admin/export?from=2020-01-01&to=2099-12-31"
# Expect: 200 or 302

curl -s -o /dev/null -w "%{http_code}" $BASE/admin/export/xlsx
# Expect: 200 or 302
```

### 10. Static Assets

```bash
curl -s -o /dev/null -w "%{http_code}" $BASE/static/lern.png
curl -s -o /dev/null -w "%{http_code}" $BASE/static/favicon.svg
curl -s -o /dev/null -w "%{http_code}" $BASE/static/js/fingerprint.js
# Expect: 200 for all
```

### 11. Task Item API

These require authentication. Use cookie-based sessions:

```bash
curl -s -c /tmp/cg-cookies -X POST $BASE/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}'
# Expect: {"ok": true, "role": "admin"}

# Create global tracker item
curl -s -b /tmp/cg-cookies -X POST $BASE/api/v1/tracker/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"Daily Math Quiz","priority":"high","recurrence":"daily","category":"Math"}'
# Expect: {"ok": true, "id": N}

# List global tracker items
curl -s -b /tmp/cg-cookies $BASE/api/v1/tracker/items
# Expect: JSON array with at least 1 item

# Create personal task (assigned to student)
curl -s -b /tmp/cg-cookies -X POST $BASE/api/tracker/student-items \
  -H 'Content-Type: application/json' \
  -d '{"student_id":"S001","name":"Homework Ch5","priority":"medium","recurrence":"none","requires_signoff":true}'
# Expect: {"ok": true, "id": N}

# Due items for student
curl -s -b /tmp/cg-cookies "$BASE/api/tracker/due?student_id=S001"
# Expect: JSON array with global + student-specific items

# All tasks for student
curl -s -b /tmp/cg-cookies "$BASE/api/dashboard/all-tasks?student_id=S001"
# Expect: {"global_items": [...], "student_items": [...]}

rm -f /tmp/cg-cookies
```

### 12. Column Preferences API

These require authentication. Reuse cookies from test 11.

```bash
# Login as admin
curl -s -c /tmp/cg-cookies -X POST $BASE/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}'

# GET preferences — initially empty
curl -s -b /tmp/cg-cookies $BASE/api/v1/preferences
# Expect: {} (empty object)

# POST column visibility preferences
curl -s -b /tmp/cg-cookies -X POST $BASE/api/v1/preferences \
  -H 'Content-Type: application/json' \
  -d '{"data_columns":"{\"students\":{\"id\":true,\"first_name\":true,\"last_name\":true,\"grade\":true,\"school\":false}}"}'
# Expect: {"ok": true}

# GET preferences — should return saved data
curl -s -b /tmp/cg-cookies $BASE/api/v1/preferences
# Expect: {"data_columns": "..."} with the saved JSON string

# Unauthenticated request should fail
curl -s -o /dev/null -w "%{http_code}" $BASE/api/v1/preferences
# Expect: 302 (redirect to login) or 401

rm -f /tmp/cg-cookies
```

### 13. Go Test Suite

```bash
go test -v -count=1 .
# Expect: all tests PASS

go test -v -count=1 ./internal/scheduling
# Expect: all scheduling tests PASS
```

## Reporting

Print a summary table after all tests:

```
Validation Results:
  Pages:           PASS (mobile, kiosk, login, admin, dashboard)
  APIs:            PASS (settings, status, search)
  Check-In Flow:   PASS (checkin, duplicate, checkout, status)
  Rejection:       PASS (unknown, inactive)
  Kiosk:           PASS (checkin, checkout)
  Student ID:      PASS (checkin by ID)
  Fingerprint:     PASS (capture)
  Attendance:      PASS (multi-student list)
  Task Items:      PASS (global CRUD, personal, due, all-tasks)
  Exports:         PASS (CSV, XLSX)
  Static:          PASS (logo, favicon, JS)
  Preferences:     PASS (save, load, unauthenticated rejected)
  Go Tests:        PASS (all passed)
```

If any test fails, show expected vs actual.
