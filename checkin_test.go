package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/handlers"
)

// setupTestWithData creates a test App with CSV example data imported and a rate limiter.
func setupTestWithData(t *testing.T) (*handlers.App, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "classgo-e2e-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()

	db, err := database.OpenDB(dbPath)
	if err != nil {
		os.Remove(dbPath)
		t.Fatal(err)
	}

	if err := database.MigrateDB(db); err != nil {
		db.Close()
		os.Remove(dbPath)
		t.Fatal(err)
	}

	// Import CSV example data
	tmpDataDir, err := os.MkdirTemp("", "classgo-data-*")
	if err != nil {
		db.Close()
		os.Remove(dbPath)
		t.Fatal(err)
	}

	// Symlink csv.example as csv/ inside the temp data dir
	csvExamplePath, _ := filepath.Abs("data/csv.example")
	os.Symlink(csvExamplePath, filepath.Join(tmpDataDir, "csv"))

	data, err := datastore.ReadAll(tmpDataDir)
	if err != nil {
		t.Fatalf("Failed to read CSV data: %v", err)
	}
	if err := datastore.ImportAll(db, data); err != nil {
		t.Fatalf("Failed to import data: %v", err)
	}
	if len(data.Students) == 0 {
		t.Fatal("No students loaded from CSV example data")
	}

	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		db.Close()
		os.Remove(dbPath)
		os.RemoveAll(tmpDataDir)
		t.Fatal(err)
	}

	app := &handlers.App{
		DB:          db,
		Tmpl:        tmpl,
		AppName:     "TestApp",
		PinMode:     "off",
		RateLimiter: handlers.NewRateLimiter(),
	}

	return app, func() {
		db.Close()
		os.Remove(dbPath)
		os.RemoveAll(tmpDataDir)
	}
}

func checkin(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/checkin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func checkout(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func apiPost(handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func apiGet(handler http.HandlerFunc, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func resp(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var r map[string]any
	if err := json.NewDecoder(w.Body).Decode(&r); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, w.Body.String())
	}
	return r
}

func respArray(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var r []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&r); err != nil {
		t.Fatalf("failed to decode array response: %v", err)
	}
	return r
}

// ==================== GROUP 1: PIN MODE = OFF ====================

func TestPinOff_MobileCheckinNoPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("expected ok, got: %v", r)
	}
	if !strings.Contains(r["message"].(string), "Alice Wang") {
		t.Errorf("expected welcome message with name, got: %s", r["message"])
	}
}

func TestPinOff_KioskCheckinNoPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	w := checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","student_id":"S002","device_type":"kiosk"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("expected ok, got: %v", r)
	}
}

func TestPinOff_CheckoutNoPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile"}`)
	w := checkout(app.HandleCheckOut, `{"student_name":"Alice Wang","student_id":"S001"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("checkout failed: %v", r)
	}
	if !strings.Contains(r["message"].(string), "Goodbye") {
		t.Errorf("expected goodbye message, got: %s", r["message"])
	}
}

// ==================== GROUP 2: PIN MODE = CENTER ====================

func TestPinCenter_MobileCheckinWithPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "center"
	app.SetRequirePIN(true)
	app.SetPIN("5678")

	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","pin":"5678","device_type":"mobile"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("expected ok with correct PIN, got: %v", r)
	}
}

func TestPinCenter_MobileCheckinWrongPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "center"
	app.SetRequirePIN(true)
	app.SetPIN("5678")

	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","pin":"9999","device_type":"mobile"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong PIN, got %d", w.Code)
	}
}

func TestPinCenter_MobileCheckinMissingPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "center"
	app.SetRequirePIN(true)
	app.SetPIN("5678")

	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile"}`)
	r := resp(t, w)
	if r["ok"] == true {
		t.Fatal("expected failure for missing PIN")
	}
	if r["needs_pin"] != true {
		t.Errorf("expected needs_pin=true, got: %v", r)
	}
}

func TestPinCenter_KioskCheckinWithPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "center"
	app.SetRequirePIN(true)
	app.SetPIN("4321")

	w := checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","student_id":"S002","pin":"4321","device_type":"kiosk"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("kiosk check-in with correct PIN failed: %v", r)
	}
}

func TestPinCenter_CheckoutWithPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "center"
	app.SetRequirePIN(true)
	app.SetPIN("1111")

	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","pin":"1111","device_type":"mobile"}`)
	w := checkout(app.HandleCheckOut, `{"student_name":"Alice Wang","pin":"1111"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("checkout with correct PIN failed: %v", r)
	}
}

// ==================== GROUP 3: ADMIN-CONTROLLED PER-STUDENT PIN ====================

func TestPinPerStudent_FirstCheckinNoHash(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Flag student — system auto-generates PIN
	database.SetStudentRequirePIN(app.DB, "S001", true)

	// Check in without PIN — should be told PIN is required
	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile"}`)
	r := resp(t, w)
	if r["needs_pin"] != true {
		t.Fatalf("expected needs_pin=true for flagged student without PIN, got: %v", r)
	}
}

func TestPinPerStudent_SetupAndCheckin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Flag student and get admin-generated PIN
	database.SetStudentRequirePIN(app.DB, "S001", true)
	pin, err := database.EnsureDailyStudentPin(app.DB, "S001")
	if err != nil {
		t.Fatal(err)
	}

	// Check in with admin-generated PIN
	w := checkin(app.HandleCheckIn, fmt.Sprintf(`{"student_name":"Alice Wang","student_id":"S001","pin":"%s","device_type":"mobile"}`, pin))
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("check-in with admin-generated PIN failed: %v", r)
	}
}

func TestPinPerStudent_WrongPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	database.SetStudentRequirePIN(app.DB, "S001", true)
	database.EnsureDailyStudentPin(app.DB, "S001")

	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","pin":"0000","device_type":"mobile"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong personal PIN, got %d", w.Code)
	}
}

func TestPinPerStudent_AdminResetPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Flag student and get PIN
	database.SetStudentRequirePIN(app.DB, "S002", true)
	oldPin, _ := database.EnsureDailyStudentPin(app.DB, "S002")

	// Verify old PIN works
	w := checkin(app.HandleCheckIn, fmt.Sprintf(`{"student_name":"Bob Wang","student_id":"S002","pin":"%s","device_type":"kiosk"}`, oldPin))
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("pre-reset check-in failed: %v", r)
	}

	// Check out
	checkout(app.HandleCheckOut, fmt.Sprintf(`{"student_name":"Bob Wang","student_id":"S002","pin":"%s"}`, oldPin))

	// Admin regenerates PIN
	apiPost(app.HandleStudentPINReset, "/api/v1/student/pin/reset", `{"student_id":"S002"}`)

	// Old PIN should no longer work
	w = checkin(app.HandleCheckIn, fmt.Sprintf(`{"student_name":"Bob Wang","student_id":"S002","pin":"%s","device_type":"kiosk"}`, oldPin))
	if w.Code != http.StatusUnauthorized {
		r = resp(t, w)
		if r["ok"] == true {
			t.Fatalf("old PIN should not work after admin reset")
		}
	}

	// New PIN should work
	newPin, _, _ := database.GetStudentPin(app.DB, "S002")
	w = checkin(app.HandleCheckIn, fmt.Sprintf(`{"student_name":"Bob Wang","student_id":"S002","pin":"%s","device_type":"kiosk"}`, newPin))
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("new PIN after reset should work, got: %v", r)
	}
}

// ==================== GROUP 4: PER-STUDENT OVERRIDE ====================

func TestPinOverride_FlaggedStudentNeedsPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off" // Global mode is off

	// Flag S006 (Frank) as requiring PIN — auto-generates a PIN
	database.SetStudentRequirePIN(app.DB, "S006", true)

	// Frank should be told PIN is required (no PIN provided)
	w := checkin(app.HandleCheckIn, `{"student_name":"Frank Miller","student_id":"S006","device_type":"mobile"}`)
	r := resp(t, w)
	if r["needs_pin"] != true {
		t.Fatalf("flagged student should need PIN, got: %v", r)
	}
}

func TestPinOverride_FlaggedStudentWithPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	database.SetStudentRequirePIN(app.DB, "S006", true)
	// Get the admin-generated PIN
	pin, err := database.EnsureDailyStudentPin(app.DB, "S006")
	if err != nil || pin == "" {
		t.Fatal("expected PIN to be generated for flagged student")
	}

	// Frank checks in with the admin-generated PIN — should work
	w := checkin(app.HandleCheckIn, fmt.Sprintf(`{"student_name":"Frank Miller","student_id":"S006","pin":"%s","device_type":"mobile"}`, pin))
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("flagged student with correct PIN should succeed, got: %v", r)
	}
}

func TestPinOverride_UnflaggedStudentNoPin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// S001 (Alice) is NOT flagged — should check in freely
	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("unflagged student should check in without PIN, got: %v", r)
	}
}

// ==================== GROUP 4b: PIN OVERRIDE API HANDLERS ====================

func TestPinCheck_OffModeUnflagged(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	w := apiGet(app.HandlePINCheck, "/api/pin/check?student_id=S001")
	r := resp(t, w)
	if r["needs_pin"] != false {
		t.Errorf("expected needs_pin=false for unflagged student in off mode, got: %v", r)
	}
	if r["pin_mode"] != "off" {
		t.Errorf("expected pin_mode=off, got: %v", r["pin_mode"])
	}
}

func TestPinCheck_OffModeFlagged(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	database.SetStudentRequirePIN(app.DB, "S001", true)

	w := apiGet(app.HandlePINCheck, "/api/pin/check?student_id=S001")
	r := resp(t, w)
	if r["needs_pin"] != true {
		t.Errorf("expected needs_pin=true for flagged student, got: %v", r)
	}
}

func TestPinCheck_CenterMode(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "center"

	w := apiGet(app.HandlePINCheck, "/api/pin/check?student_id=S001")
	r := resp(t, w)
	if r["needs_pin"] != true {
		t.Errorf("expected needs_pin=true in center mode, got: %v", r)
	}
	if r["pin_mode"] != "center" {
		t.Errorf("expected pin_mode=center, got: %v", r["pin_mode"])
	}
}

func TestPinCheck_ByStudentName(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	database.SetStudentRequirePIN(app.DB, "S001", true)

	w := apiGet(app.HandlePINCheck, "/api/pin/check?student_name=Alice+Wang")
	r := resp(t, w)
	if r["needs_pin"] != true {
		t.Errorf("expected needs_pin=true when looking up by name, got: %v", r)
	}
}

func TestHandleStudentRequirePIN_FlagAndUnflag(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Flag student
	w := apiPost(app.HandleStudentRequirePIN, "/api/v1/student/pin/require",
		`{"student_id":"S003","require_pin":true}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("flag student failed: %v", r)
	}
	if r["pin"] == nil || r["pin"] == "" {
		t.Error("expected PIN to be returned when flagging student")
	}

	// Verify student is flagged
	if !database.StudentRequiresPIN(app.DB, "S003") {
		t.Error("student should be flagged after API call")
	}

	// Unflag student
	w = apiPost(app.HandleStudentRequirePIN, "/api/v1/student/pin/require",
		`{"student_id":"S003","require_pin":false}`)
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("unflag student failed: %v", r)
	}

	// Verify student is unflagged
	if database.StudentRequiresPIN(app.DB, "S003") {
		t.Error("student should not be flagged after unflagging")
	}
}

func TestHandlePINModeChange(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()

	// Change to center
	w := apiPost(app.HandlePINModeChange, "/api/admin/pin/mode",
		`{"pin_mode":"center"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("set center mode failed: %v", r)
	}
	if app.PinMode != "center" {
		t.Errorf("expected PinMode=center, got %s", app.PinMode)
	}

	// Change to off
	w = apiPost(app.HandlePINModeChange, "/api/admin/pin/mode",
		`{"pin_mode":"off"}`)
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("set off mode failed: %v", r)
	}
	if app.PinMode != "off" {
		t.Errorf("expected PinMode=off, got %s", app.PinMode)
	}

	// Invalid mode
	w = apiPost(app.HandlePINModeChange, "/api/admin/pin/mode",
		`{"pin_mode":"invalid"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid mode, got %d", w.Code)
	}
}

func TestHandleStudentPINReset(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()

	// Flag and generate initial PIN
	database.SetStudentRequirePIN(app.DB, "S004", true)
	oldPin, _ := database.EnsureDailyStudentPin(app.DB, "S004")

	// Reset PIN via handler
	w := apiPost(app.HandleStudentPINReset, "/api/v1/student/pin/reset",
		`{"student_id":"S004"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("PIN reset failed: %v", r)
	}
	newPin, ok := r["pin"].(string)
	if !ok || newPin == "" {
		t.Fatal("expected new PIN in response")
	}
	if newPin == oldPin {
		t.Error("new PIN should differ from old PIN")
	}
}

// ==================== GROUP 5: RATE LIMITING ====================

func TestRateLimit_MobileDifferentStudents(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Alice checks in from device
	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile","fingerprint":"fp1","device_id":"dev1"}`)

	// Bob tries from same device immediately — should be rate limited
	w := checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","student_id":"S002","device_type":"mobile","fingerprint":"fp1","device_id":"dev1"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for rapid different-student check-in, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRateLimit_SameStudentAllowed(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Alice checks in
	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile","fingerprint":"fp1","device_id":"dev1"}`)

	// Alice checks in again (duplicate) — should NOT be rate limited (same student)
	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile","fingerprint":"fp1","device_id":"dev1"}`)
	if w.Code == http.StatusTooManyRequests {
		t.Error("same student re-checking-in should not be rate limited")
	}
	r := resp(t, w)
	if r["ok"] != true {
		t.Errorf("expected ok (duplicate allowed), got: %v", r)
	}
}

func TestRateLimit_KioskShorterCooldown(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	// Alice checks in from kiosk
	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"kiosk","fingerprint":"kiosk-fp","device_id":"kiosk"}`)

	// Bob tries from same kiosk immediately — should be rate limited (30s)
	w := checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","student_id":"S002","device_type":"kiosk","fingerprint":"kiosk-fp","device_id":"kiosk"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for rapid kiosk different-student, got %d", w.Code)
	}
}

// ==================== GROUP 6: AUDIT & DASHBOARD ====================

func TestAudit_CheckinCreatesAuditRecord(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile","fingerprint":"test-fp","device_id":"test-dev"}`)

	// Query audit table directly
	var count int
	app.DB.QueryRow("SELECT COUNT(*) FROM checkin_audit WHERE student_name = 'Alice Wang' AND action = 'checkin'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 audit record, got %d", count)
	}

	var clientIP, fingerprint, deviceID string
	app.DB.QueryRow("SELECT client_ip, COALESCE(fingerprint,''), COALESCE(device_id,'') FROM checkin_audit WHERE student_name = 'Alice Wang'").Scan(&clientIP, &fingerprint, &deviceID)
	if clientIP == "" {
		t.Error("expected client_ip to be captured")
	}
	if fingerprint != "test-fp" {
		t.Errorf("expected fingerprint=test-fp, got %s", fingerprint)
	}
	if deviceID != "test-dev" {
		t.Errorf("expected device_id=test-dev, got %s", deviceID)
	}
}

func TestAudit_BuddyPunchFlagged(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"
	// Disable rate limiter for this test so we can trigger the audit flag
	app.RateLimiter = nil

	// Alice checks in from device X
	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile","fingerprint":"shared-fp","device_id":"shared-dev"}`)
	// Bob checks in from same device within 5 minutes
	checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","student_id":"S002","device_type":"mobile","fingerprint":"shared-fp","device_id":"shared-dev"}`)

	// Check for flagged records
	var flagged int
	app.DB.QueryRow("SELECT COUNT(*) FROM checkin_audit WHERE flagged = 1").Scan(&flagged)
	if flagged == 0 {
		t.Error("expected at least one flagged audit record for buddy punching")
	}

	var reason string
	app.DB.QueryRow("SELECT flag_reason FROM checkin_audit WHERE flagged = 1 LIMIT 1").Scan(&reason)
	if !strings.Contains(reason, "multiple students") {
		t.Errorf("expected flag reason about multiple students, got: %s", reason)
	}
}

func TestDashboard_AttendeesAfterCheckin(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	checkin(app.HandleCheckIn, `{"student_name":"Carlos Garcia","student_id":"S003","device_type":"mobile"}`)

	w := apiGet(app.HandleAttendees, "/api/attendees")
	attendees := respArray(t, w)
	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
	if attendees[0]["student_name"] != "Carlos Garcia" {
		t.Errorf("expected Carlos Garcia, got %s", attendees[0]["student_name"])
	}
	if attendees[0]["check_in_time"] == "" {
		t.Error("expected check_in_time to be set")
	}
	if attendees[0]["date"] == "" {
		t.Error("expected date field to be set")
	}
}

func TestDashboard_MetricsAfterCheckins(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"
	app.RateLimiter = nil // disable for multi-student

	checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","device_type":"mobile"}`)
	checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","device_type":"kiosk"}`)
	checkin(app.HandleCheckIn, `{"student_name":"Carlos Garcia","device_type":"mobile"}`)

	today := time.Now().Format("2006-01-02")
	w := apiGet(app.HandleAttendanceMetrics, "/api/attendees/metrics?from="+today+"&to="+today)
	r := resp(t, w)

	totalCheckins := int(r["total_checkins"].(float64))
	uniqueStudents := int(r["unique_students"].(float64))
	if totalCheckins != 3 {
		t.Errorf("expected 3 total_checkins, got %d", totalCheckins)
	}
	if uniqueStudents != 3 {
		t.Errorf("expected 3 unique_students, got %d", uniqueStudents)
	}
}

func TestDashboard_AttendeesByDateRange(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"

	checkin(app.HandleCheckIn, `{"student_name":"Diana Chen","device_type":"mobile"}`)

	today := time.Now().Format("2006-01-02")
	w := apiGet(app.HandleAttendees, "/api/attendees?from="+today+"&to="+today)
	attendees := respArray(t, w)
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee with date range filter, got %d", len(attendees))
	}

	// Future date should return empty
	w = apiGet(app.HandleAttendees, "/api/attendees?from=2099-01-01&to=2099-01-02")
	attendees = respArray(t, w)
	if len(attendees) != 0 {
		t.Errorf("expected 0 attendees for future date, got %d", len(attendees))
	}
}

// ==================== GROUP 7: FULL E2E FLOW ====================

func TestFullE2E_MultiStudentFlow(t *testing.T) {
	app, cleanup := setupTestWithData(t)
	defer cleanup()
	app.PinMode = "off"
	app.RateLimiter = nil // disable for multi-student test

	// Check in 3 students
	w := checkin(app.HandleCheckIn, `{"student_name":"Alice Wang","student_id":"S001","device_type":"mobile"}`)
	r := resp(t, w)
	if r["ok"] != true {
		t.Fatalf("Alice check-in failed: %v", r)
	}

	w = checkin(app.HandleCheckIn, `{"student_name":"Bob Wang","student_id":"S002","device_type":"kiosk"}`)
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("Bob check-in failed: %v", r)
	}

	w = checkin(app.HandleCheckIn, `{"student_name":"Carlos Garcia","student_id":"S003","device_type":"mobile"}`)
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("Carlos check-in failed: %v", r)
	}

	// Verify all 3 in attendees
	w = apiGet(app.HandleAttendees, "/api/attendees")
	attendees := respArray(t, w)
	if len(attendees) != 3 {
		t.Fatalf("expected 3 attendees, got %d", len(attendees))
	}

	// Verify status for each
	for _, name := range []string{"Alice Wang", "Bob Wang", "Carlos Garcia"} {
		w = apiGet(app.HandleStatus, "/api/status?student_name="+strings.ReplaceAll(name, " ", "%20"))
		r = resp(t, w)
		if r["checked_in"] != true {
			t.Errorf("%s should be checked in", name)
		}
		if r["checked_out"] != false {
			t.Errorf("%s should not be checked out yet", name)
		}
	}

	// Check out Alice and Bob
	time.Sleep(100 * time.Millisecond) // small delay for duration
	w = checkout(app.HandleCheckOut, `{"student_name":"Alice Wang"}`)
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("Alice checkout failed: %v", r)
	}

	w = checkout(app.HandleCheckOut, `{"student_name":"Bob Wang"}`)
	r = resp(t, w)
	if r["ok"] != true {
		t.Fatalf("Bob checkout failed: %v", r)
	}

	// Verify attendees — all 3 should be there, 2 with check_out_time
	w = apiGet(app.HandleAttendees, "/api/attendees")
	attendees = respArray(t, w)
	if len(attendees) != 3 {
		t.Fatalf("expected 3 attendees after checkouts, got %d", len(attendees))
	}

	checkedOut := 0
	for _, a := range attendees {
		if a["check_out_time"] != nil && a["check_out_time"] != "" {
			checkedOut++
			if a["duration"] == nil || a["duration"] == "" {
				t.Errorf("checked-out student %s should have duration", a["student_name"])
			}
		}
	}
	if checkedOut != 2 {
		t.Errorf("expected 2 checked-out students, got %d", checkedOut)
	}

	// Carlos should still be active
	w = apiGet(app.HandleStatus, "/api/status?student_name=Carlos%20Garcia")
	r = resp(t, w)
	if r["checked_in"] != true || r["checked_out"] != false {
		t.Error("Carlos should still be checked in (not out)")
	}

	// Verify metrics
	today := time.Now().Format("2006-01-02")
	w = apiGet(app.HandleAttendanceMetrics, "/api/attendees/metrics?from="+today+"&to="+today)
	r = resp(t, w)
	if int(r["total_checkins"].(float64)) != 3 {
		t.Errorf("expected 3 total_checkins, got %v", r["total_checkins"])
	}
	if int(r["total_checkouts"].(float64)) != 2 {
		t.Errorf("expected 2 total_checkouts, got %v", r["total_checkouts"])
	}
	if int(r["unique_students"].(float64)) != 3 {
		t.Errorf("expected 3 unique_students, got %v", r["unique_students"])
	}
}
