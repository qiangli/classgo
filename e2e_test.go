package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	if resp["redirect"] != "/home" {
		t.Errorf("expected redirect to /home after login, got %v", resp["redirect"])
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
		if it["item_type"] == "personal" && it["name"] == "Complete Math Worksheet" {
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
			{"item_type":"personal","item_id":` + jsonNum(signoffItemID) + `,"item_name":"Complete Math Worksheet","status":"done"}
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
		Recurrence: "none", Active: true, Type: models.TaskTypeTask,
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
		Recurrence: "none", Active: true, Type: models.TaskTypeTodo,
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
		Recurrence: "none", Active: true, Type: models.TaskTypeTodo, CreatedBy: "admin",
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
			if m["type"] != "todo" {
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
			if it.Name == "Weekly Progress Report" && it.RequiresSignoff() {
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

// ====================================================================================
// Unified Task Items E2E: center, class, and personal scoped items
// ====================================================================================

func TestTaskItems_CenterScope_DueForAllStudents(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a center-scoped signoff item
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "Daily Homework Check",
		Priority: "high", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
		CreatedBy: "admin", OwnerType: "admin",
	})

	// Both students should see it
	for _, sid := range []string{"S001", "S002"} {
		items, err := database.GetDueItems(app.DB, sid, today())
		if err != nil {
			t.Fatalf("GetDueItems(%s): %v", sid, err)
		}
		found := false
		for _, it := range items {
			if it.Name == "Daily Homework Check" && it.Scope == models.ScopeCenter && it.ItemType == "global" {
				found = true
			}
		}
		if !found {
			t.Errorf("student %s should see center-scoped item 'Daily Homework Check'", sid)
		}
	}
}

func TestTaskItems_ClassScope_OnlyEnrolledStudents(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a schedule with S001 enrolled, S002 NOT enrolled
	app.DB.Exec(`INSERT INTO schedules (id, day_of_week, start_time, end_time, teacher_id, subject, student_ids, deleted)
		VALUES ('SCH_TEST', 'Monday', '15:00', '16:00', 'T01', 'Math', 'S001', 0)`)

	// Create a class-scoped signoff item for that schedule
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeClass, ScheduleID: "SCH_TEST",
		Name: "Math Class Worksheet", Priority: "high", Recurrence: "daily",
		Type: models.TaskTypeTodo, Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})

	// S001 (enrolled) should see it
	items, _ := database.GetDueItems(app.DB, "S001", today())
	found := false
	for _, it := range items {
		if it.Name == "Math Class Worksheet" && it.Scope == models.ScopeClass {
			found = true
		}
	}
	if !found {
		t.Error("enrolled student S001 should see class-scoped item")
	}

	// S002 (not enrolled) should NOT see it
	items, _ = database.GetDueItems(app.DB, "S002", today())
	for _, it := range items {
		if it.Name == "Math Class Worksheet" {
			t.Error("unenrolled student S002 should NOT see class-scoped item")
		}
	}
}

func TestTaskItems_PersonalScope_OnlyAssignedStudent(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopePersonal, StudentID: "S001",
		Name: "Alice Homework", Priority: "high", Recurrence: "none",
		Type: models.TaskTypeTodo, Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})

	// S001 should see it
	items, _ := database.GetDueItems(app.DB, "S001", today())
	found := false
	for _, it := range items {
		if it.Name == "Alice Homework" && it.Scope == models.ScopePersonal {
			found = true
		}
	}
	if !found {
		t.Error("S001 should see personal item 'Alice Homework'")
	}

	// S002 should NOT see it
	items, _ = database.GetDueItems(app.DB, "S002", today())
	for _, it := range items {
		if it.Name == "Alice Homework" {
			t.Error("S002 should NOT see S001's personal item")
		}
	}
}

func TestTaskItems_MixedScopeCheckout(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// Create schedule with S001 enrolled
	app.DB.Exec(`INSERT INTO schedules (id, day_of_week, start_time, end_time, teacher_id, subject, student_ids, deleted)
		VALUES ('SCH_MIX', 'Monday', '15:00', '16:00', 'T01', 'Math', 'S001', 0)`)

	// Center item (signoff)
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "Center Task",
		Priority: "high", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
		CreatedBy: "admin", OwnerType: "admin",
	})
	// Class item (signoff)
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeClass, ScheduleID: "SCH_MIX",
		Name: "Class Task", Priority: "high", Recurrence: "daily",
		Type: models.TaskTypeTodo, Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})
	// Personal item (signoff)
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopePersonal, StudentID: "S001",
		Name: "Personal Task", Priority: "high", Recurrence: "none",
		Type: models.TaskTypeTodo, Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})

	// Check in S001
	postJSON(app.HandleCheckIn, `{"student_name":"Alice","device_type":"mobile"}`)

	// Checkout should be blocked by signoff items
	w := postJSON(app.HandleCheckOut, `{"student_name":"Alice","student_id":"S001"}`)
	resp := decodeResp(t, w)
	if resp["ok"] == true {
		t.Fatal("checkout should be blocked by 3 signoff items")
	}
	if resp["pending_tasks"] != true {
		t.Error("expected pending_tasks=true")
	}

	// Check that pending items include all 3 scopes
	pending, _ := database.PendingSignoffItems(app.DB, "S001")
	names := map[string]bool{}
	for _, p := range pending {
		names[p.Name] = true
	}
	for _, expected := range []string{"Center Task", "Class Task", "Personal Task"} {
		if !names[expected] {
			t.Errorf("expected pending item '%s' not found", expected)
		}
	}
}

func TestTaskItems_AccessControl_CompleteItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a personal item for S001
	itemID, _ := database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopePersonal, StudentID: "S001",
		Name: "S001 Only Task", Priority: "medium", Recurrence: "none",
		Type: models.TaskTypeTodo, Active: true, CreatedBy: "admin", OwnerType: "admin",
	})

	// S002 should NOT be able to complete S001's item
	req := reqWithSession("POST", "/api/tracker/complete",
		`{"id":`+jsonNum(float64(itemID))+`,"complete":true}`,
		app, "user", "student", "S002")
	w := doReq(app.HandleTrackerComplete, req)
	resp := mustDecode(t, w)
	if resp["ok"] == true {
		t.Error("S002 should not be able to complete S001's item")
	}

	// S001 should be able to complete their own item
	req = reqWithSession("POST", "/api/tracker/complete",
		`{"id":`+jsonNum(float64(itemID))+`,"complete":true}`,
		app, "user", "student", "S001")
	w = doReq(app.HandleTrackerComplete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("S001 should be able to complete own item: %v", resp)
	}
}

func TestTaskItems_BulkAssign_ParentBlocked(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Parent should NOT be able to bulk-assign
	req := reqWithSession("POST", "/api/dashboard/bulk-assign",
		`{"student_ids":["S001"],"name":"Parent Assigned Task"}`,
		app, "user", "parent", "P001")
	w := doReq(app.HandleTrackerBulkAssign, req)
	resp := mustDecode(t, w)
	if resp["ok"] == true {
		t.Error("parent should not be able to bulk-assign")
	}
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func TestTaskItems_CriteriaFiltering(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// S001 is grade 6, S002 is grade 7 (from seed data)
	// Update S001 to grade 10 for this test
	app.DB.Exec("UPDATE students SET grade = '10' WHERE id = 'S001'")
	app.DB.Exec("UPDATE students SET grade = '6' WHERE id = 'S002'")

	// Create a center item with grade_min=9 criteria
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "AP Prep Item",
		Type: models.TaskTypeTodo, Priority: "high", Recurrence: "daily",
		Criteria: `{"grade_min": 9}`, Active: true,
		CreatedBy: "admin", OwnerType: "admin",
	})

	// S001 (grade 10) should see it
	items, _ := database.GetDueItems(app.DB, "S001", today())
	found := false
	for _, it := range items {
		if it.Name == "AP Prep Item" {
			found = true
		}
	}
	if !found {
		t.Error("grade 10 student should see item with grade_min=9")
	}

	// S002 (grade 6) should NOT see it
	items, _ = database.GetDueItems(app.DB, "S002", today())
	for _, it := range items {
		if it.Name == "AP Prep Item" {
			t.Error("grade 6 student should NOT see item with grade_min=9")
		}
	}
}

func TestTaskItems_TypeFiltering_CheckoutOnlyTodo(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create one of each type
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "Required Todo",
		Type: models.TaskTypeTodo, Priority: "high", Recurrence: "daily",
		Active: true, CreatedBy: "admin", OwnerType: "admin",
	})
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "Optional Task",
		Type: models.TaskTypeTask, Priority: "medium", Recurrence: "daily",
		Active: true, CreatedBy: "admin", OwnerType: "admin",
	})
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "Info Reminder",
		Type: models.TaskTypeReminder, Priority: "low", Recurrence: "daily",
		Active: true, CreatedBy: "admin", OwnerType: "admin",
	})

	// All three should appear in GetDueItems
	allDue, _ := database.GetDueItems(app.DB, "S001", today())
	if len(allDue) != 3 {
		t.Fatalf("expected 3 due items, got %d", len(allDue))
	}

	// PendingSignoffItems should only return the todo
	pending, _ := database.PendingSignoffItems(app.DB, "S001")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending signoff item, got %d", len(pending))
	}
	if pending[0].Name != "Required Todo" {
		t.Errorf("expected 'Required Todo', got %q", pending[0].Name)
	}
	if pending[0].Type != models.TaskTypeTodo {
		t.Errorf("expected type=todo, got %q", pending[0].Type)
	}
}

func TestTaskItems_LateSignoff(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a daily todo item
	itemID, _ := database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopeCenter, Name: "Daily Check",
		Type: models.TaskTypeTodo, Priority: "high", Recurrence: "daily",
		Active: true, CreatedBy: "admin", OwnerType: "admin",
	})

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Record late signoff for yesterday
	req := reqWithSession("POST", "/api/tracker/late-signoff",
		`{"student_id":"S001","item_id":`+jsonNum(float64(itemID))+`,"due_date":"`+yesterday+`","status":"done","notes":"completed late"}`,
		app, "admin", "admin", "admin")
	w := doReq(app.HandleLateSignoff, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("late signoff failed: %v", resp)
	}

	// Verify the response was recorded with is_late=1
	var isLate int
	var dueDate string
	app.DB.QueryRow("SELECT is_late, due_date FROM tracker_responses WHERE student_id = 'S001' AND item_name = 'Daily Check'").Scan(&isLate, &dueDate)
	if isLate != 1 {
		t.Errorf("expected is_late=1, got %d", isLate)
	}
	if dueDate != yesterday {
		t.Errorf("expected due_date=%s, got %s", yesterday, dueDate)
	}

	// The item should no longer be due for yesterday (late signoff covers it)
	items, _ := database.GetDueItems(app.DB, "S001", yesterday)
	for _, it := range items {
		if it.Name == "Daily Check" {
			t.Error("item should not be due for yesterday after late signoff")
		}
	}

	// But should still be due for today
	items, _ = database.GetDueItems(app.DB, "S001", today())
	found := false
	for _, it := range items {
		if it.Name == "Daily Check" {
			found = true
		}
	}
	if !found {
		t.Error("item should still be due for today")
	}
}

func TestTaskItems_TaskGroups(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a task group
	app.DB.Exec(`INSERT INTO task_groups (id, name, min_required, enforce_order) VALUES ('essay-s001', 'College Essay', NULL, 1)`)

	// Create grouped items
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopePersonal, StudentID: "S001",
		Name: "First Draft", Type: models.TaskTypeTodo,
		Priority: "high", Recurrence: "none",
		GroupID: "essay-s001", GroupOrder: 1,
		Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopePersonal, StudentID: "S001",
		Name: "Second Draft", Type: models.TaskTypeTodo,
		Priority: "high", Recurrence: "none",
		GroupID: "essay-s001", GroupOrder: 2,
		Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})
	database.SaveTaskItem(app.DB, models.TaskItem{
		Scope: models.ScopePersonal, StudentID: "S001",
		Name: "Final Submission", Type: models.TaskTypeTodo,
		Priority: "high", Recurrence: "none",
		GroupID: "essay-s001", GroupOrder: 3,
		Active: true, CreatedBy: "T01", OwnerType: "teacher",
	})

	// All three should show as due
	items, _ := database.GetDueItems(app.DB, "S001", today())
	groupItems := []models.DueItem{}
	for _, it := range items {
		if it.GroupID == "essay-s001" {
			groupItems = append(groupItems, it)
		}
	}
	if len(groupItems) != 3 {
		t.Fatalf("expected 3 group items, got %d", len(groupItems))
	}

	// Verify group_id and group_order are populated
	for _, it := range groupItems {
		if it.GroupID != "essay-s001" {
			t.Errorf("expected group_id=essay-s001, got %s", it.GroupID)
		}
		if it.GroupOrder == 0 {
			t.Error("expected non-zero group_order")
		}
	}
}
