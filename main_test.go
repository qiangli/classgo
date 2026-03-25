package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTest(t *testing.T) func() {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "classgo-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()

	// Open test database
	var openErr error
	db, openErr = openDB(dbPath)
	if openErr != nil {
		os.Remove(dbPath)
		t.Fatal(openErr)
	}

	if err := migrateDB(); err != nil {
		db.Close()
		os.Remove(dbPath)
		t.Fatal(err)
	}

	// Set a known PIN
	mu.Lock()
	dailyPIN = "1234"
	pinDate = time.Now().Format("2006-01-02")
	mu.Unlock()

	appName = "TestApp"

	// Parse templates
	tmpl, err = template.ParseGlob("templates/*.html")
	if err != nil {
		db.Close()
		os.Remove(dbPath)
		t.Fatal(err)
	}

	return func() {
		db.Close()
		os.Remove(dbPath)
	}
}

func postJSON(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func getJSON(handler http.HandlerFunc, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func decodeResp(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v, body: %s", err, w.Body.String())
	}
	return resp
}

func TestSignInMobile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	beforeSignIn := time.Now()

	// Sign in
	w := postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %v", w.Code, resp)
	}
	if resp["ok"] != true {
		t.Fatalf("sign-in failed: %v", resp)
	}
	if msg, _ := resp["message"].(string); !strings.Contains(msg, "Alice") {
		t.Fatalf("expected welcome message with name, got: %s", msg)
	}

	// Verify attendees
	w = getJSON(handleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)

	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}

	a := attendees[0]
	if a["student_name"] != "Alice" {
		t.Errorf("expected student_name=Alice, got %v", a["student_name"])
	}
	if a["device_type"] != "mobile" {
		t.Errorf("expected device_type=mobile, got %v", a["device_type"])
	}

	// Validate sign_in_time is a correctly formatted local time string
	signInStr, ok := a["sign_in_time"].(string)
	if !ok || signInStr == "" {
		t.Fatalf("sign_in_time is missing or empty: %v", a["sign_in_time"])
	}

	// Parse the formatted time string (e.g., "3:04 PM")
	parsedSignIn, err := time.Parse("3:04 PM", signInStr)
	if err != nil {
		t.Fatalf("failed to parse sign_in_time %q: %v", signInStr, err)
	}

	// Check the hour/minute match the current time (within a 2-minute window)
	nowH, nowM, _ := beforeSignIn.Clock()
	parsedH, parsedM, _ := parsedSignIn.Clock()
	diffMin := (nowH*60 + nowM) - (parsedH*60 + parsedM)
	if diffMin < 0 {
		diffMin = -diffMin
	}
	if diffMin > 2 {
		t.Errorf("sign_in_time %q is too far from current time %02d:%02d (diff=%d min)", signInStr, nowH, nowM, diffMin)
	}

	// sign_out_time should be empty
	if a["sign_out_time"] != "" {
		t.Errorf("expected empty sign_out_time, got %v", a["sign_out_time"])
	}
	if a["duration"] != "" {
		t.Errorf("expected empty duration, got %v", a["duration"])
	}
}

func TestSignInKiosk(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(handleSignIn, `{"student_name":"Bob","pin":"1234","device_type":"kiosk"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("sign-in failed: %v", resp)
	}

	w = getJSON(handleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)

	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
	if attendees[0]["device_type"] != "kiosk" {
		t.Errorf("expected device_type=kiosk, got %v", attendees[0]["device_type"])
	}
}

func TestDuplicateSignIn(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)

	// Second sign-in should say "already"
	w := postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("expected ok=true for duplicate, got: %v", resp)
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(strings.ToLower(msg), "already") {
		t.Errorf("expected 'already' message, got: %s", msg)
	}

	// Should still be only 1 record
	w = getJSON(handleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee after duplicate sign-in, got %d", len(attendees))
	}
}

func TestSignOut(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Sign in first
	postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)

	// Sign out
	w := postJSON(handleSignOut, `{"student_name":"Alice","pin":"1234"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("sign-out failed: %v", resp)
	}
	if msg, _ := resp["message"].(string); !strings.Contains(msg, "Alice") {
		t.Errorf("expected goodbye message with name, got: %s", msg)
	}

	// Verify attendees shows sign-out time and duration
	w = getJSON(handleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)

	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}

	a := attendees[0]
	signOutStr, _ := a["sign_out_time"].(string)
	if signOutStr == "" {
		t.Fatal("sign_out_time should not be empty after sign-out")
	}

	// Validate sign_out_time format
	_, err := time.Parse("3:04 PM", signOutStr)
	if err != nil {
		t.Errorf("failed to parse sign_out_time %q: %v", signOutStr, err)
	}

	// Duration should be set (even if 0m)
	dur, _ := a["duration"].(string)
	if dur == "" {
		t.Error("duration should not be empty after sign-out")
	}
	if !strings.Contains(dur, "m") {
		t.Errorf("duration should contain 'm' (minutes), got: %s", dur)
	}
}

func TestSignOutWithoutSignIn(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(handleSignOut, `{"student_name":"Nobody","pin":"1234"}`)
	resp := decodeResp(t, w)

	if resp["ok"] == true {
		t.Fatal("sign-out should fail when not signed in")
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(strings.ToLower(errMsg), "no active") {
		t.Errorf("expected 'no active sign-in' error, got: %s", errMsg)
	}
}

func TestSignOutThenSignInAgain(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Sign in
	postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	// Sign out
	postJSON(handleSignOut, `{"student_name":"Alice","pin":"1234"}`)

	// Sign in again — should succeed (not "already signed in")
	w := postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("second sign-in failed: %v", resp)
	}
	msg, _ := resp["message"].(string)
	if strings.Contains(strings.ToLower(msg), "already") {
		t.Error("should allow sign-in after sign-out, but got 'already' message")
	}

	// Should now be 2 records
	w = getJSON(handleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 2 {
		t.Errorf("expected 2 attendees after sign-out + re-sign-in, got %d", len(attendees))
	}
}

func TestStatus(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Not signed in
	w := getJSON(handleStatus, "/api/status?student_name=Alice")
	resp := decodeResp(t, w)
	if resp["signed_in"] != false {
		t.Error("expected signed_in=false before sign-in")
	}

	// Sign in
	postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)

	// Should be signed in, not signed out
	w = getJSON(handleStatus, "/api/status?student_name=Alice")
	resp = decodeResp(t, w)
	if resp["signed_in"] != true {
		t.Error("expected signed_in=true after sign-in")
	}
	if resp["signed_out"] != false {
		t.Error("expected signed_out=false before sign-out")
	}

	// Sign out
	postJSON(handleSignOut, `{"student_name":"Alice","pin":"1234"}`)

	w = getJSON(handleStatus, "/api/status?student_name=Alice")
	resp = decodeResp(t, w)
	if resp["signed_in"] != true {
		t.Error("expected signed_in=true after sign-out (record exists)")
	}
	if resp["signed_out"] != true {
		t.Error("expected signed_out=true after sign-out")
	}
}

func TestInvalidPIN(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(handleSignIn, `{"student_name":"Alice","pin":"9999","device_type":"mobile"}`)
	if w.Code != 401 {
		t.Errorf("expected 401 for bad PIN, got %d", w.Code)
	}

	w = postJSON(handleSignOut, `{"student_name":"Alice","pin":"9999"}`)
	if w.Code != 401 {
		t.Errorf("expected 401 for bad PIN on sign-out, got %d", w.Code)
	}
}

func TestMissingFields(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Missing name
	w := postJSON(handleSignIn, `{"pin":"1234","device_type":"mobile"}`)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}

	// Missing PIN
	w = postJSON(handleSignIn, `{"student_name":"Alice","device_type":"mobile"}`)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing PIN, got %d", w.Code)
	}
}

func TestFullFlowBothDeviceTypes(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	now := time.Now()

	// Mobile sign-in
	w := postJSON(handleSignIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("mobile sign-in failed: %v", resp)
	}

	// Kiosk sign-in
	w = postJSON(handleSignIn, `{"student_name":"Bob","pin":"1234","device_type":"kiosk"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("kiosk sign-in failed: %v", resp)
	}

	// Both signed in
	w = getJSON(handleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(attendees))
	}

	// Validate times for both
	for _, a := range attendees {
		name := a["student_name"].(string)
		signInStr := a["sign_in_time"].(string)

		parsed, err := time.Parse("3:04 PM", signInStr)
		if err != nil {
			t.Errorf("%s: invalid sign_in_time %q: %v", name, signInStr, err)
			continue
		}

		// Verify the time is close to now
		pH, pM, _ := parsed.Clock()
		nH, nM, _ := now.Clock()
		diff := (nH*60 + nM) - (pH*60 + pM)
		if diff < 0 {
			diff = -diff
		}
		if diff > 2 {
			t.Errorf("%s: sign_in_time %q differs from current time %02d:%02d by %d min", name, signInStr, nH, nM, diff)
		}

		if a["sign_out_time"] != "" {
			t.Errorf("%s: expected empty sign_out_time before sign-out", name)
		}
	}

	// Sign out both
	w = postJSON(handleSignOut, `{"student_name":"Alice","pin":"1234"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("Alice sign-out failed: %v", resp)
	}

	w = postJSON(handleSignOut, `{"student_name":"Bob","pin":"1234"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("Bob sign-out failed: %v", resp)
	}

	// Verify all have sign-out times and durations
	w = getJSON(handleAttendees, "/api/attendees")
	json.NewDecoder(w.Body).Decode(&attendees)

	for _, a := range attendees {
		name := a["student_name"].(string)

		signOutStr, _ := a["sign_out_time"].(string)
		if signOutStr == "" {
			t.Errorf("%s: sign_out_time should not be empty", name)
		}
		_, err := time.Parse("3:04 PM", signOutStr)
		if err != nil {
			t.Errorf("%s: invalid sign_out_time %q: %v", name, signOutStr, err)
		}

		dur, _ := a["duration"].(string)
		if dur == "" {
			t.Errorf("%s: duration should not be empty", name)
		}
	}
}
