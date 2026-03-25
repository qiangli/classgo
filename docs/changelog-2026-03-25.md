# Changes — 2026-03-25

## Summary

Simplified student sign-in by removing the student ID requirement, added sign-out
support with time tracking, and improved the kiosk UI for tablet use.

## Changes

### 1. Remove Student ID Requirement

- Students now sign in with **name + PIN** only — no student ID field.
- Database schema updated: dropped `student_id` and `timestamp` columns, replaced
  with `sign_in_time` and `sign_out_time`.
- Duplicate detection uses `student_name` instead of `student_id`.
- Mobile, kiosk, and admin templates updated accordingly.

### 2. Sign-In / Sign-Out Time Tracking

- New `sign_out_time` column (nullable) in the attendance table.
- New `POST /api/signout` endpoint — requires `student_name` and `pin`.
- `GET /api/status` now returns `signed_out` boolean alongside `signed_in`.
- Mobile: signed-in confirmation card shows a **Sign Out** button (prompts for PIN).
- Kiosk: name entry step shows both **Sign In** and **Sign Out** buttons.
- Kiosk sign-out validates the student is currently signed in before proceeding.
- After sign-out, students can sign in again (creates a new attendance record).

### 3. Duration on Admin Page

- Admin attendance table columns: Name, Device, Sign In, Sign Out, Duration.
- Active sessions show "active" in green; completed sessions show sign-out time
  and calculated duration (e.g. "1h 23m").
- CSV export includes Sign In, Sign Out, and Duration columns.
- Auto-refresh (5s polling) updates all fields including sign-out and duration.

### 4. Timestamp Parsing Fix

- Fixed incorrect sign-in/sign-out times caused by `modernc.org/sqlite` returning
  `datetime('now','localtime')` values as RFC3339 with a `Z` (UTC) suffix.
- The driver's `Z` is misleading — the value is already local time.
- New `parseTimestamp()` function strips the timezone artifact and parses as local.
- Integration tests validate that displayed times match the actual wall clock.

### 5. Browser Cache Fix

- Added `noCache` middleware on all page and API routes.
- Sets `Cache-Control: no-cache, no-store, must-revalidate`, `Pragma: no-cache`,
  and `Expires: 0`.
- Static assets (`/static/`) remain cacheable.

### 6. QR Code: IP + mDNS Toggle

- QR codes now support both IP address and mDNS (`hostname.local`) URLs.
- Default shows the mDNS URL; tapping the QR image toggles to the IP address.
- URL label below the QR updates on toggle.
- Available on both admin and kiosk pages.
- Startup log prints both URLs.

### 7. Kiosk UI Scaled for Tablet

- Container widened (`max-w-2xl`), all elements scaled up for tablet screens.
- Logo 16x16, title 6xl, QR 44x44, inputs/buttons text-2xl with larger padding,
  keypad buttons 2rem font, PIN display 6xl, success overlays 10rem icon.
- QR URL displayed below the QR image (was missing).

### 8. Integration Tests + Make Target

- Added `main_test.go` with 10 integration tests covering:
  - Sign-in for mobile and kiosk device types
  - Duplicate sign-in prevention
  - Sign-out with time and duration validation
  - Sign-out without prior sign-in (error case)
  - Sign-out then re-sign-in flow
  - Status endpoint state transitions
  - Invalid PIN rejection
  - Missing field validation
  - Full end-to-end flow for both device types with time correctness checks
- Refactored `initDB()` into `openDB()` + `migrateDB()` for test isolation.
- Added `make test` target in Makefile.

## Files Changed

| File | Change |
|------|--------|
| `main.go` | Schema, API, timestamp parsing, no-cache middleware, mDNS support |
| `templates/mobile.html` | Removed student ID field, added sign-out button |
| `templates/kiosk.html` | Removed student ID step, sign-out flow, tablet scaling, QR URL |
| `templates/admin.html` | Sign-out/duration columns, QR toggle, JS refresh update |
| `main_test.go` | New — 10 integration tests |
| `Makefile` | Added `test` target |
