package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/models"
)

// ====================================================================================
// Phase 1 E2E: Simple check-in/check-out with minimal user profile (signup)
// No IDs required — students sign up with first name + last name + password,
// then check in/out by name.
// ====================================================================================

func TestPhase1_SignupLoginCheckinCheckout(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// 1. Signup with just first name, last name, and password
	w := signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("signup failed: %v", resp)
	}
	if resp["redirect"] != "/profile" {
		t.Errorf("expected redirect to /profile after signup, got %v", resp["redirect"])
	}

	// 2. Login with the entity_id returned (simulates second visit)
	w = signupJSON(app, `{"action":"login","entity_id":"S010","password":"pass1234"}`)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("login failed: %v", resp)
	}
	if resp["redirect"] != "/dashboard" {
		t.Errorf("expected redirect to /dashboard after login, got %v", resp["redirect"])
	}

	// 3. Check in (by name, no PIN required in Phase 1)
	w = postJSON(app.HandleCheckIn, `{"student_name":"Grace Lee","device_type":"mobile"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-in failed: %v", resp)
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "Grace Lee") {
		t.Errorf("expected welcome message with name, got: %s", msg)
	}

	// 4. Verify checked-in status
	w = getJSON(app.HandleStatus, "/api/status?student_name=Grace+Lee")
	resp = decodeResp(t, w)
	if resp["checked_in"] != true {
		t.Error("expected checked_in=true")
	}
	if resp["checked_out"] != false {
		t.Error("expected checked_out=false before checkout")
	}

	// 5. No due items (Phase 1 — no tasks assigned)
	w = getJSON(app.HandleTrackerDue, "/api/tracker/due?student_id=S010")
	var dueItems []map[string]any
	json.NewDecoder(w.Body).Decode(&dueItems)
	if len(dueItems) != 0 {
		t.Errorf("Phase 1: expected 0 due items, got %d", len(dueItems))
	}

	// 6. Check out (no signoff needed — no pending tasks)
	w = postJSON(app.HandleCheckOut, `{"student_name":"Grace Lee"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-out failed: %v", resp)
	}

	// 7. Verify checked-out status
	w = getJSON(app.HandleStatus, "/api/status?student_name=Grace+Lee")
	resp = decodeResp(t, w)
	if resp["checked_in"] != true {
		t.Error("expected checked_in=true (record exists)")
	}
	if resp["checked_out"] != true {
		t.Error("expected checked_out=true")
	}

	// 8. Verify attendees list shows the session
	w = getJSON(app.HandleAttendees, "/api/attendees")
	var attendees []map[string]any
	json.NewDecoder(w.Body).Decode(&attendees)
	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
	if attendees[0]["student_name"] != "Grace Lee" {
		t.Errorf("expected student_name=Grace Lee, got %v", attendees[0]["student_name"])
	}
	if attendees[0]["check_out_time"] == "" {
		t.Error("expected check_out_time to be set")
	}
}

func TestPhase1_CheckinWithoutSignup(t *testing.T) {
	// Students in the system can check in without having signed up for an account
	app, cleanup := setupSignupTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	w := postJSON(app.HandleCheckIn, `{"student_name":"Grace Lee","device_type":"kiosk"}`)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-in without signup should work: %v", resp)
	}

	w = postJSON(app.HandleCheckOut, `{"student_name":"Grace Lee"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-out without signup should work: %v", resp)
	}
}

func TestPhase1_UnknownStudentRejected(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	w := postJSON(app.HandleCheckIn, `{"student_name":"Unknown Person","device_type":"mobile"}`)
	if w.Code != 400 {
		t.Errorf("expected 400 for unknown student, got %d", w.Code)
	}
	resp := decodeResp(t, w)
	if resp["ok"] != false {
		t.Error("expected ok=false for unknown student")
	}
}

// ====================================================================================
// Phase 2 E2E: Admin creates tasks, assigns to students, checkout requires signoff
// ====================================================================================

func TestPhase2_AdminCreateTaskAndAssignToStudent(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// 1. Admin creates a student-specific task with requires_signoff
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Complete Math Worksheet","priority":"high","recurrence":"none","requires_signoff":true,"active":true}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create student item failed: %v", resp)
	}
	signoffItemID := resp["id"].(float64)

	// 2. Student checks in
	w = postJSON(app.HandleCheckIn, `{"student_name":"Alice","device_type":"mobile"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-in failed: %v", resp)
	}

	// 3. Student sees the assigned task in due items
	w = getJSON(app.HandleTrackerDue, "/api/tracker/due?student_id=S001")
	var dueItems []map[string]any
	json.NewDecoder(w.Body).Decode(&dueItems)
	if len(dueItems) == 0 {
		t.Fatal("expected due items for student")
	}
	foundAdhoc := false
	for _, it := range dueItems {
		if it["item_type"] == "adhoc" && it["name"] == "Complete Math Worksheet" {
			foundAdhoc = true
		}
	}
	if !foundAdhoc {
		t.Error("expected 'Complete Math Worksheet' in due items")
	}

	// 4. Checkout is BLOCKED because of pending signoff task
	w = postJSON(app.HandleCheckOut, `{"student_name":"Alice"}`)
	resp = decodeResp(t, w)
	if resp["ok"] == true {
		t.Fatal("checkout should be blocked when there are pending signoff tasks")
	}
	if resp["pending_tasks"] != true {
		t.Error("expected pending_tasks=true in blocked checkout response")
	}
	pendingItems, _ := resp["items"].([]any)
	if len(pendingItems) == 0 {
		t.Error("expected pending items list in blocked checkout response")
	}

	// 5. Student responds to tasks via /api/tracker/respond (this also performs checkout)
	respondBody := `{
		"student_name": "Alice",
		"student_id": "S001",
		"responses": [
			{"item_type":"adhoc","item_id":` + jsonNum(signoffItemID) + `,"item_name":"Complete Math Worksheet","status":"done"}
		]}`
	w = postJSON(app.HandleTrackerRespond, respondBody)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("tracker respond + checkout failed: %v", resp)
	}

	// 6. Verify checked out
	w = getJSON(app.HandleStatus, "/api/status?student_name=Alice")
	resp = decodeResp(t, w)
	if resp["checked_out"] != true {
		t.Error("expected checked_out=true after tracker respond")
	}

	// 7. Verify the response was recorded
	req = reqWithSession("GET", "/api/v1/tracker/responses?student_id=S001", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerResponses, req)
	var responses []map[string]any
	json.NewDecoder(w.Body).Decode(&responses)
	found := false
	for _, r := range responses {
		if r["item_name"] == "Complete Math Worksheet" && r["status"] == "done" {
			found = true
		}
	}
	if !found {
		t.Error("expected response record for 'Complete Math Worksheet'")
	}
}

func TestPhase2_CheckoutAllowedWhenNoSignoffTasks(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// Create a student task WITHOUT requires_signoff
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Optional Reading", Priority: "low",
		Recurrence: "none", Active: true, RequiresSignoff: false,
	})

	// Check in
	w := postJSON(app.HandleCheckIn, `{"student_name":"Alice","device_type":"mobile"}`)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-in failed: %v", resp)
	}

	// Checkout should succeed — no signoff-required tasks
	w = postJSON(app.HandleCheckOut, `{"student_name":"Alice"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("checkout should succeed without signoff tasks: %v", resp)
	}
}

func TestPhase2_CheckoutBlockedUntilSignoffComplete(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// Create a signoff-required task
	itemID, _ := database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Required Essay", Priority: "high",
		Recurrence: "none", Active: true, RequiresSignoff: true,
	})

	// Check in
	postJSON(app.HandleCheckIn, `{"student_name":"Alice","device_type":"mobile"}`)

	// Checkout blocked
	w := postJSON(app.HandleCheckOut, `{"student_name":"Alice"}`)
	resp := decodeResp(t, w)
	if resp["ok"] == true {
		t.Fatal("checkout should be blocked")
	}
	if resp["pending_tasks"] != true {
		t.Error("expected pending_tasks=true")
	}

	// Student marks the task as complete
	app.Sessions = auth.NewSessionStore()
	req := reqWithSession("POST", "/api/tracker/complete",
		`{"id":`+jsonNum(float64(itemID))+`,"complete":true,"entity_id":"S001"}`,
		app, "user", "student", "S001")
	w = doReq(app.HandleTrackerComplete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("complete failed: %v", resp)
	}

	// Now checkout succeeds (completed items are no longer due)
	w = postJSON(app.HandleCheckOut, `{"student_name":"Alice"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("checkout should succeed after completing signoff task: %v", resp)
	}
}

func TestPhase2_UserLoginSeesDueTasks(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)
	app.Sessions = auth.NewSessionStore()

	// Seed tracker items
	database.SeedSampleData(app.DB)

	// Create a signoff task for the student
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S010", Name: "Complete Profile", Priority: "high",
		Recurrence: "none", Active: true, RequiresSignoff: true, CreatedBy: "admin",
	})

	// Signup
	w := signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("signup failed: %v", resp)
	}

	// After login, user can see their due tasks via dashboard
	req := reqWithSession("GET", "/api/dashboard/all-tasks?student_id=S010", "", app, "user", "student", "S010")
	w = doReq(app.HandleDashboardAllTasks, req)
	resp = mustDecode(t, w)

	studentItems, _ := resp["student_items"].([]any)
	if len(studentItems) == 0 {
		t.Error("expected student to see due tasks after login")
	}

	// Verify the assigned task is in the list
	found := false
	for _, item := range studentItems {
		m, _ := item.(map[string]any)
		if m["name"] == "Complete Profile" {
			found = true
			if m["requires_signoff"] != true {
				t.Error("expected requires_signoff=true for the assigned task")
			}
		}
	}
	if !found {
		t.Error("expected 'Complete Profile' task in student's task list")
	}
}

func TestPhase2_BulkAssignFromLibrary(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// 1. Admin creates a library item (template)
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"name":"Weekly Progress Report","priority":"medium","recurrence":"weekly","requires_signoff":true}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	libID := resp["id"].(float64)

	// 2. Admin assigns to multiple students
	req = reqWithSession("POST", "/api/dashboard/assign-library-item",
		`{"item_id":`+jsonNum(libID)+`,"student_ids":["S001","S002"]}`,
		app, "admin", "", "admin")
	w = doReq(app.HandleAssignLibraryItem, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("assign failed: %v", resp)
	}
	if resp["count"].(float64) != 2 {
		t.Errorf("expected count=2, got %v", resp["count"])
	}

	// 3. Both students should have the task
	for _, sid := range []string{"S001", "S002"} {
		items, _ := database.ListStudentTrackerItems(app.DB, sid)
		found := false
		for _, it := range items {
			if it.Name == "Weekly Progress Report" && it.RequiresSignoff {
				found = true
			}
		}
		if !found {
			t.Errorf("student %s should have 'Weekly Progress Report' with signoff required", sid)
		}
	}

	// 4. Check in both students
	postJSON(app.HandleCheckIn, `{"student_name":"Alice","device_type":"mobile"}`)
	postJSON(app.HandleCheckIn, `{"student_name":"Bob","device_type":"kiosk"}`)

	// 5. Both should be blocked from direct checkout
	for _, name := range []string{"Alice", "Bob"} {
		w = postJSON(app.HandleCheckOut, `{"student_name":"`+name+`"}`)
		resp = decodeResp(t, w)
		if resp["ok"] == true {
			t.Errorf("%s: checkout should be blocked by pending signoff task", name)
		}
		if resp["pending_tasks"] != true {
			t.Errorf("%s: expected pending_tasks=true", name)
		}
	}
}

// ====================================================================================
// Column Preferences E2E: Admin saves column visibility, preferences persist and
// are isolated per user.
// ====================================================================================

func TestColumnPreferences_SaveAndLoad(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// 1. GET preferences for admin — should be empty initially
	req := reqWithSession("GET", "/api/v1/preferences", "", app, "admin", "", "admin")
	w := doReq(app.HandlePreferences, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET preferences: expected 200, got %d", w.Code)
	}
	var prefs map[string]string
	json.NewDecoder(w.Body).Decode(&prefs)
	if len(prefs) != 0 {
		t.Errorf("expected empty preferences initially, got %v", prefs)
	}

	// 2. POST column visibility preferences (value must be a string, not nested JSON)
	colPrefs := `{"students":{"id":true,"first_name":true,"last_name":true,"grade":true,"school":false,"parent_id":true,"email":false,"phone":false,"active":true}}`
	// The preferences API expects map[string]string, so data_columns value is a JSON string
	postBody, _ := json.Marshal(map[string]string{"data_columns": colPrefs})
	req = reqWithSession("POST", "/api/v1/preferences",
		string(postBody),
		app, "admin", "", "admin")
	w = doReq(app.HandlePreferences, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST preferences: expected 200, got %d", w.Code)
	}
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("save preferences failed: %v", resp)
	}

	// 3. GET preferences — should return saved data
	req = reqWithSession("GET", "/api/v1/preferences", "", app, "admin", "", "admin")
	w = doReq(app.HandlePreferences, req)
	prefs = map[string]string{}
	json.NewDecoder(w.Body).Decode(&prefs)
	if prefs["data_columns"] == "" {
		t.Fatal("expected data_columns in preferences")
	}
	var savedCols map[string]map[string]bool
	json.Unmarshal([]byte(prefs["data_columns"]), &savedCols)
	if savedCols["students"]["school"] != false {
		t.Error("expected school=false in saved preferences")
	}
	if savedCols["students"]["id"] != true {
		t.Error("expected id=true in saved preferences")
	}

	// 4. Update preferences (toggle a column)
	colPrefs2 := `{"students":{"id":true,"first_name":true,"last_name":true,"grade":true,"school":true,"parent_id":true,"email":true,"phone":false,"active":true}}`
	postBody, _ = json.Marshal(map[string]string{"data_columns": colPrefs2})
	req = reqWithSession("POST", "/api/v1/preferences",
		string(postBody),
		app, "admin", "", "admin")
	w = doReq(app.HandlePreferences, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("update preferences failed: %v", resp)
	}

	// 5. Verify update persisted
	req = reqWithSession("GET", "/api/v1/preferences", "", app, "admin", "", "admin")
	w = doReq(app.HandlePreferences, req)
	prefs = map[string]string{}
	json.NewDecoder(w.Body).Decode(&prefs)
	json.Unmarshal([]byte(prefs["data_columns"]), &savedCols)
	if savedCols["students"]["school"] != true {
		t.Error("expected school=true after update")
	}
	if savedCols["students"]["email"] != true {
		t.Error("expected email=true after update")
	}

	// 6. Different user has separate preferences
	req = reqWithSession("GET", "/api/v1/preferences", "", app, "user", "student", "S001")
	w = doReq(app.HandlePreferences, req)
	var studentPrefs map[string]string
	json.NewDecoder(w.Body).Decode(&studentPrefs)
	if len(studentPrefs) != 0 {
		t.Errorf("expected empty preferences for different user, got %v", studentPrefs)
	}
}

// ====================================================================================
// Admin Data Page Round-Trip: Navigate to student profile and back.
// Verifies admin page, profile page, and directory API all work in sequence,
// simulating the user flow: admin data tab → student profile → back to admin data.
// ====================================================================================

func TestAdminDataPage_ReturnFromProfile(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// 1. Admin loads the admin page (initial visit)
	req := reqWithSession("GET", "/admin", "", app, "admin", "", "admin")
	w := doReq(app.HandleAdmin, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin page: expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "section-data") {
		t.Error("admin page should contain data section")
	}
	if !strings.Contains(body, `navigate(sections.includes(hash)`) {
		t.Error("admin page should contain hash-based navigation init")
	}

	// 2. Directory API returns student data (what the data tab fetches via JS)
	req = reqWithSession("GET", "/api/v1/directory", "", app, "admin", "", "admin")
	w = doReq(app.HandleDirectoryAPI, req)
	if w.Code != http.StatusOK {
		t.Fatalf("directory API: expected 200, got %d", w.Code)
	}
	var dirData map[string]any
	json.NewDecoder(w.Body).Decode(&dirData)
	students, ok := dirData["students"].([]any)
	if !ok || len(students) == 0 {
		t.Fatal("directory API should return students")
	}

	// 3. Admin navigates to a student profile page
	req = reqWithSession("GET", "/admin/profile?id=S001", "", app, "admin", "", "admin")
	w = doReq(app.HandleProfilePage, req)
	if w.Code != http.StatusOK {
		t.Fatalf("profile page: expected 200, got %d", w.Code)
	}
	profileBody := w.Body.String()
	if !strings.Contains(profileBody, "/admin#data") {
		t.Error("profile page should contain back link to /admin#data")
	}

	// 4. Profile API returns student data
	req = reqWithSession("GET", "/api/v1/student/profile?id=S001", "", app, "admin", "", "admin")
	w = doReq(app.HandleStudentProfile, req)
	if w.Code != http.StatusOK {
		t.Fatalf("student profile API: expected 200, got %d", w.Code)
	}
	var profileResp map[string]any
	json.NewDecoder(w.Body).Decode(&profileResp)
	if profileResp["ok"] != true {
		t.Fatalf("student profile API failed: %v", profileResp)
	}

	// 5. Simulate returning to admin page (as if user clicked /admin#data back link)
	// The admin page must render successfully again
	req = reqWithSession("GET", "/admin", "", app, "admin", "", "admin")
	w = doReq(app.HandleAdmin, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin page on return: expected 200, got %d", w.Code)
	}

	// 6. Directory API must still return data after the round-trip
	req = reqWithSession("GET", "/api/v1/directory", "", app, "admin", "", "admin")
	w = doReq(app.HandleDirectoryAPI, req)
	if w.Code != http.StatusOK {
		t.Fatalf("directory API on return: expected 200, got %d", w.Code)
	}
	dirData = map[string]any{}
	json.NewDecoder(w.Body).Decode(&dirData)
	students, ok = dirData["students"].([]any)
	if !ok || len(students) == 0 {
		t.Fatal("directory API should still return students after round-trip")
	}

	// 7. Preferences API must work (used by data tab init)
	req = reqWithSession("GET", "/api/v1/preferences", "", app, "admin", "", "admin")
	w = doReq(app.HandlePreferences, req)
	if w.Code != http.StatusOK {
		t.Fatalf("preferences API on return: expected 200, got %d", w.Code)
	}
}

func TestColumnPreferences_Unauthenticated(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Request without session — should get 401
	req := httptest.NewRequest("GET", "/api/v1/preferences", nil)
	w := doReq(app.HandlePreferences, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}
