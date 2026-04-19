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

	"classgo/internal/database"
	"classgo/internal/handlers"
)

func setupTest(t *testing.T) (*handlers.App, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "classgo-test-*.db")
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

	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		db.Close()
		os.Remove(dbPath)
		t.Fatal(err)
	}

	app := &handlers.App{
		DB:      db,
		Tmpl:    tmpl,
		AppName: "TestApp",
	}
	app.SetPIN("1234")
	app.SetRequirePIN(true)

	return app, func() {
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

func TestCheckInMobile(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	beforeCheckIn := time.Now()

	w := postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %v", w.Code, resp)
	}
	if resp["ok"] != true {
		t.Fatalf("check-in failed: %v", resp)
	}
	if msg, _ := resp["message"].(string); !strings.Contains(msg, "Alice") {
		t.Fatalf("expected welcome message with name, got: %s", msg)
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
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

	checkInStr, ok := a["check_in_time"].(string)
	if !ok || checkInStr == "" {
		t.Fatalf("check_in_time is missing or empty: %v", a["check_in_time"])
	}

	parsedCheckIn, err := time.Parse("3:04 PM", checkInStr)
	if err != nil {
		t.Fatalf("failed to parse check_in_time %q: %v", checkInStr, err)
	}

	nowH, nowM, _ := beforeCheckIn.Clock()
	parsedH, parsedM, _ := parsedCheckIn.Clock()
	diffMin := (nowH*60 + nowM) - (parsedH*60 + parsedM)
	if diffMin < 0 {
		diffMin = -diffMin
	}
	if diffMin > 2 {
		t.Errorf("check_in_time %q is too far from current time %02d:%02d (diff=%d min)", checkInStr, nowH, nowM, diffMin)
	}

	if a["check_out_time"] != "" {
		t.Errorf("expected empty check_out_time, got %v", a["check_out_time"])
	}
	if a["duration"] != "" {
		t.Errorf("expected empty duration, got %v", a["duration"])
	}
}

func TestCheckInKiosk(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(app.HandleCheckIn, `{"student_name":"Bob","pin":"1234","device_type":"kiosk"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("check-in failed: %v", resp)
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)

	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
	if attendees[0]["device_type"] != "kiosk" {
		t.Errorf("expected device_type=kiosk, got %v", attendees[0]["device_type"])
	}
}

func TestDuplicateCheckIn(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)

	w := postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("expected ok=true for duplicate, got: %v", resp)
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(strings.ToLower(msg), "already") {
		t.Errorf("expected 'already' message, got: %s", msg)
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 1 {
		t.Errorf("expected 1 attendee after duplicate check-in, got %d", len(attendees))
	}
}

func TestCheckOut(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)

	w := postJSON(app.HandleCheckOut, `{"student_name":"Alice","pin":"1234"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("check-out failed: %v", resp)
	}
	if msg, _ := resp["message"].(string); !strings.Contains(msg, "Alice") {
		t.Errorf("expected goodbye message with name, got: %s", msg)
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)

	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}

	a := attendees[0]
	checkOutStr, _ := a["check_out_time"].(string)
	if checkOutStr == "" {
		t.Fatal("check_out_time should not be empty after check-out")
	}

	_, err := time.Parse("3:04 PM", checkOutStr)
	if err != nil {
		t.Errorf("failed to parse check_out_time %q: %v", checkOutStr, err)
	}

	dur, _ := a["duration"].(string)
	if dur == "" {
		t.Error("duration should not be empty after check-out")
	}
	if !strings.Contains(dur, "m") {
		t.Errorf("duration should contain 'm' (minutes), got: %s", dur)
	}
}

func TestCheckOutWithoutCheckIn(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(app.HandleCheckOut, `{"student_name":"Nobody","pin":"1234"}`)
	resp := decodeResp(t, w)

	if resp["ok"] == true {
		t.Fatal("check-out should fail when not signed in")
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(strings.ToLower(errMsg), "no active") {
		t.Errorf("expected 'no active check-in' error, got: %s", errMsg)
	}
}

func TestCheckOutThenCheckInAgain(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	postJSON(app.HandleCheckOut, `{"student_name":"Alice","pin":"1234"}`)

	w := postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)

	if resp["ok"] != true {
		t.Fatalf("second check-in failed: %v", resp)
	}
	msg, _ := resp["message"].(string)
	if strings.Contains(strings.ToLower(msg), "already") {
		t.Error("should allow check-in after check-out, but got 'already' message")
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 2 {
		t.Errorf("expected 2 attendees after check-out + re-check-in, got %d", len(attendees))
	}
}

func TestStatus(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	w := getJSON(app.HandleStatus, "/api/status?student_name=Alice")
	resp := decodeResp(t, w)
	if resp["checked_in"] != false {
		t.Error("expected checked_in=false before check-in")
	}

	postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)

	w = getJSON(app.HandleStatus, "/api/status?student_name=Alice")
	resp = decodeResp(t, w)
	if resp["checked_in"] != true {
		t.Error("expected checked_in=true after check-in")
	}
	if resp["checked_out"] != false {
		t.Error("expected checked_out=false before check-out")
	}

	postJSON(app.HandleCheckOut, `{"student_name":"Alice","pin":"1234"}`)

	w = getJSON(app.HandleStatus, "/api/status?student_name=Alice")
	resp = decodeResp(t, w)
	if resp["checked_in"] != true {
		t.Error("expected checked_in=true after check-out (record exists)")
	}
	if resp["checked_out"] != true {
		t.Error("expected checked_out=true after check-out")
	}
}

func TestInvalidPIN(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"9999","device_type":"mobile"}`)
	if w.Code != 401 {
		t.Errorf("expected 401 for bad PIN, got %d", w.Code)
	}

	w = postJSON(app.HandleCheckOut, `{"student_name":"Alice","pin":"9999"}`)
	if w.Code != 401 {
		t.Errorf("expected 401 for bad PIN on check-out, got %d", w.Code)
	}
}

func TestMissingFields(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	w := postJSON(app.HandleCheckIn, `{"pin":"1234","device_type":"mobile"}`)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}

	w = postJSON(app.HandleCheckIn, `{"student_name":"Alice","device_type":"mobile"}`)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing PIN, got %d", w.Code)
	}
}

func TestFullFlowBothDeviceTypes(t *testing.T) {
	app, cleanup := setupTest(t)
	defer cleanup()

	now := time.Now()

	w := postJSON(app.HandleCheckIn, `{"student_name":"Alice","pin":"1234","device_type":"mobile"}`)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("mobile check-in failed: %v", resp)
	}

	w = postJSON(app.HandleCheckIn, `{"student_name":"Bob","pin":"1234","device_type":"kiosk"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("kiosk check-in failed: %v", resp)
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(attendees))
	}

	for _, a := range attendees {
		name := a["student_name"].(string)
		checkInStr := a["check_in_time"].(string)

		parsed, err := time.Parse("3:04 PM", checkInStr)
		if err != nil {
			t.Errorf("%s: invalid check_in_time %q: %v", name, checkInStr, err)
			continue
		}

		pH, pM, _ := parsed.Clock()
		nH, nM, _ := now.Clock()
		diff := (nH*60 + nM) - (pH*60 + pM)
		if diff < 0 {
			diff = -diff
		}
		if diff > 2 {
			t.Errorf("%s: check_in_time %q differs from current time %02d:%02d by %d min", name, checkInStr, nH, nM, diff)
		}

		if a["check_out_time"] != "" {
			t.Errorf("%s: expected empty check_out_time before check-out", name)
		}
	}

	w = postJSON(app.HandleCheckOut, `{"student_name":"Alice","pin":"1234"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("Alice check-out failed: %v", resp)
	}

	w = postJSON(app.HandleCheckOut, `{"student_name":"Bob","pin":"1234"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("Bob check-out failed: %v", resp)
	}

	w = getJSON(app.HandleAttendees, "/api/attendees")
	json.NewDecoder(w.Body).Decode(&attendees)

	for _, a := range attendees {
		name := a["student_name"].(string)

		checkOutStr, _ := a["check_out_time"].(string)
		if checkOutStr == "" {
			t.Errorf("%s: check_out_time should not be empty", name)
		}
		_, err := time.Parse("3:04 PM", checkOutStr)
		if err != nil {
			t.Errorf("%s: invalid check_out_time %q: %v", name, checkOutStr, err)
		}

		dur, _ := a["duration"].(string)
		if dur == "" {
			t.Errorf("%s: duration should not be empty", name)
		}
	}
}
