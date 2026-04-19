package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/handlers"
	"classgo/internal/models"
)

// setupTrackerTest extends setupTest with session support and test data for tracker tests.
func setupTrackerTest(t *testing.T) (*handlers.App, func()) {
	t.Helper()
	app, cleanup := setupTest(t)
	app.Sessions = auth.NewSessionStore()

	// Insert additional test students, a parent, and a teacher
	for _, s := range []struct{ id, first, last string }{
		{"S003", "Carlos", "Garcia"},
		{"S004", "Diana", "Chen"},
	} {
		app.DB.Exec("INSERT OR IGNORE INTO students (id, first_name, last_name, active) VALUES (?, ?, ?, 1)", s.id, s.first, s.last)
	}
	app.DB.Exec("INSERT OR IGNORE INTO parents (id, first_name, last_name, active) VALUES ('P001', 'Parent', 'One', 1)")
	app.DB.Exec("INSERT OR IGNORE INTO teachers (id, first_name, last_name, active) VALUES ('T001', 'Teacher', 'One', 1)")
	// Link students to parent
	app.DB.Exec("UPDATE students SET parent_id = 'P001' WHERE id IN ('S001', 'S002')")
	// Create a schedule linking teacher to students
	app.DB.Exec("INSERT OR IGNORE INTO schedules (id, teacher_id, student_ids, room_id, day_of_week, start_time, end_time, deleted) VALUES ('SCH1', 'T001', 'S001;S002;S003', 'R1', 'Monday', '09:00', '10:00', 0)")
	app.DB.Exec("INSERT OR IGNORE INTO rooms (id, name, active) VALUES ('R1', 'Room 1', 1)")

	return app, cleanup
}

// reqWithSession creates an HTTP request with a session cookie.
func reqWithSession(method, path, body string, app *handlers.App, role, userType, entityID string) *http.Request {
	token := app.Sessions.Create("test-"+entityID, role, userType, entityID)
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	return req
}

func doReq(handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func mustDecode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode failed: %v, body: %s", err, w.Body.String())
	}
	return m
}

func mustDecodeArray(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var arr []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&arr); err != nil {
		t.Fatalf("decode array failed: %v, body: %s", err, w.Body.String())
	}
	return arr
}

// ==================== GLOBAL TRACKER ITEMS (Admin) ====================

func TestGlobalTrackerItem_CRUD(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a global item
	id, err := database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Daily Math Quiz", Priority: "high", Recurrence: "daily", Category: "Math", Active: true,
	})
	if err != nil {
		t.Fatalf("SaveTrackerItem: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	// List global items
	items, err := database.ListTrackerItems(app.DB, false)
	if err != nil {
		t.Fatalf("ListTrackerItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "Daily Math Quiz" {
		t.Errorf("expected name 'Daily Math Quiz', got %q", items[0].Name)
	}
	if items[0].Priority != "high" {
		t.Errorf("expected priority 'high', got %q", items[0].Priority)
	}

	// Update
	items[0].Name = "Weekly Math Quiz"
	items[0].Recurrence = "weekly"
	_, err = database.SaveTrackerItem(app.DB, items[0])
	if err != nil {
		t.Fatalf("update SaveTrackerItem: %v", err)
	}
	updated, _ := database.ListTrackerItems(app.DB, false)
	if updated[0].Name != "Weekly Math Quiz" || updated[0].Recurrence != "weekly" {
		t.Errorf("update failed: got %+v", updated[0])
	}

	// Delete (soft)
	if err := database.DeleteTrackerItem(app.DB, int(id)); err != nil {
		t.Fatalf("DeleteTrackerItem: %v", err)
	}
	afterDelete, _ := database.ListTrackerItems(app.DB, false)
	if len(afterDelete) != 0 {
		t.Errorf("expected 0 items after delete, got %d", len(afterDelete))
	}
	// Include deleted
	withDeleted, _ := database.ListTrackerItems(app.DB, true)
	if len(withDeleted) != 1 {
		t.Errorf("expected 1 item with deleted, got %d", len(withDeleted))
	}
}

func TestGlobalTrackerItem_API(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// GET — empty list
	req := reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w := doReq(app.HandleTrackerItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 0 {
		t.Fatalf("expected 0, got %d", len(items))
	}

	// POST — create
	req = reqWithSession("POST", "/api/v1/tracker/items", `{"name":"SAT Prep","priority":"high","recurrence":"weekly","category":"SAT"}`, app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create failed: %v", resp)
	}

	// GET — verify created
	req = reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	items = mustDecodeArray(t, w)
	if len(items) != 1 || items[0]["name"] != "SAT Prep" {
		t.Fatalf("expected 1 item named 'SAT Prep', got %v", items)
	}

	// DELETE
	itemID := items[0]["id"].(float64)
	req = reqWithSession("POST", "/api/v1/tracker/items/delete", `{"id":`+jsonNum(itemID)+`}`, app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItemDelete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("delete failed: %v", resp)
	}

	// Verify deleted
	req = reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	items = mustDecodeArray(t, w)
	if len(items) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(items))
	}
}

// ==================== PERSONAL TASK ITEMS (Teacher) ====================

func TestTeacherItem_CreateAssigned(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates an item assigned to a student
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Homework Ch5","priority":"medium","recurrence":"none","requires_signoff":true}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create failed: %v", resp)
	}

	// Verify it appears in the student's list
	req = reqWithSession("GET", "/api/tracker/student-items?student_id=S001", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0]["name"] != "Homework Ch5" {
		t.Errorf("expected 'Homework Ch5', got %q", items[0]["name"])
	}
	if items[0]["created_by"] != "T001" {
		t.Errorf("expected created_by=T001, got %q", items[0]["created_by"])
	}
	if items[0]["owner_type"] != "teacher" {
		t.Errorf("expected owner_type=teacher, got %q", items[0]["owner_type"])
	}
	if items[0]["requires_signoff"] != true {
		t.Errorf("expected requires_signoff=true")
	}
}

func TestTeacherItem_CreateLibrary(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates a library item (no student)
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"name":"Vocab Quiz Template","priority":"low","recurrence":"weekly","requires_signoff":true}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create library item failed: %v", resp)
	}

	// Verify via teacher-items endpoint (my items)
	req = reqWithSession("GET", "/api/dashboard/teacher-items", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleDashboardTeacherItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0]["name"] != "Vocab Quiz Template" {
		t.Errorf("expected 'Vocab Quiz Template', got %q", items[0]["name"])
	}
	if items[0]["student_id"] != "" {
		t.Errorf("expected empty student_id for library item, got %q", items[0]["student_id"])
	}
}

func TestTeacherItem_EditOwnership(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates item
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Original Name","priority":"medium","recurrence":"none"}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	itemID := resp["id"].(float64)

	// Teacher can edit own item
	req = reqWithSession("POST", "/api/tracker/student-items",
		`{"id":`+jsonNum(itemID)+`,"student_id":"S001","name":"Updated Name","priority":"high","recurrence":"none"}`,
		app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItems, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("edit failed: %v", resp)
	}

	// Verify update
	req = reqWithSession("GET", "/api/tracker/student-items?student_id=S001", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItems, req)
	items := mustDecodeArray(t, w)
	if items[0]["name"] != "Updated Name" {
		t.Errorf("expected 'Updated Name', got %q", items[0]["name"])
	}
}

func TestTeacherItem_DeleteOwnership(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher T001 creates an item
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"To Delete","priority":"medium","recurrence":"none"}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	itemID := resp["id"].(float64)

	// Another teacher cannot delete it (simulated via different entity)
	req = reqWithSession("POST", "/api/tracker/student-items/delete",
		`{"id":`+jsonNum(itemID)+`}`,
		app, "user", "teacher", "T999")
	w = doReq(app.HandleStudentTrackerItemDelete, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-owner delete, got %d", w.Code)
	}

	// Owner can delete
	req = reqWithSession("POST", "/api/tracker/student-items/delete",
		`{"id":`+jsonNum(itemID)+`}`,
		app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItemDelete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("owner delete failed: %v", resp)
	}

	// Verify gone
	req = reqWithSession("GET", "/api/tracker/student-items?student_id=S001", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(items))
	}
}

// ==================== PERSONAL TASK ITEMS (Parent) ====================

func TestParentItem_CreateForChild(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Parent creates item for their child
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Practice Piano","priority":"medium","recurrence":"daily","requires_signoff":true}`,
		app, "user", "parent", "P001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create failed: %v", resp)
	}

	// Verify via my-items endpoint
	req = reqWithSession("GET", "/api/dashboard/teacher-items", "", app, "user", "parent", "P001")
	w = doReq(app.HandleDashboardTeacherItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0]["owner_type"] != "parent" {
		t.Errorf("expected owner_type=parent, got %q", items[0]["owner_type"])
	}
}

func TestParentItem_AccessDeniedForOtherStudent(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Parent tries to view items for a student who is NOT their child
	req := reqWithSession("GET", "/api/tracker/student-items?student_id=S003", "", app, "user", "parent", "P001")
	w := doReq(app.HandleStudentTrackerItems, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-child access, got %d", w.Code)
	}
}

// ==================== PERSONAL TASK ITEMS (Student) ====================

func TestStudentItem_CreatePrivate(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Student creates a private item — student_id is auto-set
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"name":"Review Notes","priority":"low","recurrence":"none","requires_signoff":false}`,
		app, "user", "student", "S001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create failed: %v", resp)
	}

	// Verify student_id was auto-assigned
	req = reqWithSession("GET", "/api/tracker/student-items?student_id=S001", "", app, "user", "student", "S001")
	w = doReq(app.HandleStudentTrackerItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0]["student_id"] != "S001" {
		t.Errorf("expected student_id=S001, got %q", items[0]["student_id"])
	}
	if items[0]["requires_signoff"] != false {
		t.Errorf("expected requires_signoff=false for student item")
	}
	if items[0]["owner_type"] != "student" {
		t.Errorf("expected owner_type=student, got %q", items[0]["owner_type"])
	}
}

func TestStudentItem_CannotAccessOther(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Student S001 tries to view S002's items
	req := reqWithSession("GET", "/api/tracker/student-items?student_id=S002", "", app, "user", "student", "S001")
	w := doReq(app.HandleStudentTrackerItems, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-student access, got %d", w.Code)
	}
}

func TestStudentItem_CompleteOneTime(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates a one-time item for student
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Submit Essay","priority":"high","recurrence":"none","requires_signoff":true}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	itemID := resp["id"].(float64)

	// Student marks complete
	req = reqWithSession("POST", "/api/tracker/complete",
		`{"id":`+jsonNum(itemID)+`,"complete":true,"entity_id":"S001"}`,
		app, "user", "student", "S001")
	w = doReq(app.HandleTrackerComplete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("complete failed: %v", resp)
	}

	// Verify completed
	req = reqWithSession("GET", "/api/tracker/student-items?student_id=S001", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItems, req)
	items := mustDecodeArray(t, w)
	if items[0]["completed"] != true {
		t.Error("expected completed=true")
	}
	if items[0]["completed_by"] != "S001" {
		t.Errorf("expected completed_by=S001, got %q", items[0]["completed_by"])
	}

	// Uncomplete
	req = reqWithSession("POST", "/api/tracker/complete",
		`{"id":`+jsonNum(itemID)+`,"complete":false}`,
		app, "user", "student", "S001")
	w = doReq(app.HandleTrackerComplete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("uncomplete failed: %v", resp)
	}

	req = reqWithSession("GET", "/api/tracker/student-items?student_id=S001", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleStudentTrackerItems, req)
	items = mustDecodeArray(t, w)
	if items[0]["completed"] != false {
		t.Error("expected completed=false after uncomplete")
	}
}

// ==================== ASSIGN FROM LIBRARY ====================

func TestAssignLibraryItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates a library item
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"name":"Weekly Quiz","priority":"medium","recurrence":"weekly","requires_signoff":true}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	libID := resp["id"].(float64)

	// Assign to two students
	req = reqWithSession("POST", "/api/dashboard/assign-library-item",
		`{"item_id":`+jsonNum(libID)+`,"student_ids":["S001","S002"]}`,
		app, "user", "teacher", "T001")
	w = doReq(app.HandleAssignLibraryItem, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("assign failed: %v", resp)
	}
	if resp["count"].(float64) != 2 {
		t.Errorf("expected count=2, got %v", resp["count"])
	}

	// Verify items exist for both students
	for _, sid := range []string{"S001", "S002"} {
		req = reqWithSession("GET", "/api/tracker/student-items?student_id="+sid, "", app, "user", "teacher", "T001")
		w = doReq(app.HandleStudentTrackerItems, req)
		items := mustDecodeArray(t, w)
		if len(items) != 1 {
			t.Errorf("student %s: expected 1 item, got %d", sid, len(items))
			continue
		}
		if items[0]["name"] != "Weekly Quiz" {
			t.Errorf("student %s: expected 'Weekly Quiz', got %q", sid, items[0]["name"])
		}
		if items[0]["requires_signoff"] != true {
			t.Errorf("student %s: expected requires_signoff=true", sid)
		}
	}

	// Original library item still exists (unassigned)
	req = reqWithSession("GET", "/api/dashboard/teacher-items", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleDashboardTeacherItems, req)
	items := mustDecodeArray(t, w)
	// Should have 3: 1 library + 2 assigned copies
	if len(items) != 3 {
		t.Errorf("expected 3 total items (1 library + 2 assigned), got %d", len(items))
	}
	unassigned := 0
	for _, it := range items {
		if it["student_id"] == "" {
			unassigned++
		}
	}
	if unassigned != 1 {
		t.Errorf("expected 1 unassigned library item, got %d", unassigned)
	}
}

func TestAssignLibraryItem_NonOwnerDenied(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher T001 creates a library item
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"name":"My Template","priority":"low","recurrence":"none"}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	libID := resp["id"].(float64)

	// Another teacher tries to assign it
	req = reqWithSession("POST", "/api/dashboard/assign-library-item",
		`{"item_id":`+jsonNum(libID)+`,"student_ids":["S001"]}`,
		app, "user", "teacher", "T999")
	w = doReq(app.HandleAssignLibraryItem, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-owner assign, got %d", w.Code)
	}
}

// ==================== BULK ASSIGN ====================

func TestBulkAssign(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher bulk-assigns to a class schedule
	req := reqWithSession("POST", "/api/dashboard/bulk-assign",
		`{"schedule_id":"SCH1","name":"Class Homework","priority":"medium","recurrence":"none"}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleTrackerBulkAssign, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("bulk assign failed: %v", resp)
	}
	if resp["count"].(float64) != 3 {
		t.Errorf("expected count=3 (3 students in schedule), got %v", resp["count"])
	}

	// Verify each student got the item
	for _, sid := range []string{"S001", "S002", "S003"} {
		items, err := database.ListStudentTrackerItems(app.DB, sid)
		if err != nil {
			t.Fatalf("list for %s: %v", sid, err)
		}
		if len(items) != 1 || items[0].Name != "Class Homework" {
			t.Errorf("student %s: expected 'Class Homework', got %v", sid, items)
		}
	}
}

// ==================== DUE ITEMS ====================

func TestDueItems_RecurrenceFiltering(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a global daily item
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Daily Warmup", Priority: "medium", Recurrence: "daily", Active: true,
	})
	// Create a one-time student item
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "One-time Essay", Priority: "high", Recurrence: "none", Active: true, RequiresSignoff: true,
	})

	// Get due items for S001 today
	req := reqWithSession("GET", "/api/tracker/due?student_id=S001", "", app, "user", "student", "S001")
	w := doReq(app.HandleTrackerDue, req)
	var items []map[string]any
	json.NewDecoder(w.Body).Decode(&items)

	if len(items) != 2 {
		t.Fatalf("expected 2 due items (1 global + 1 adhoc), got %d", len(items))
	}

	types := map[string]bool{}
	for _, it := range items {
		types[it["item_type"].(string)] = true
	}
	if !types["global"] || !types["adhoc"] {
		t.Errorf("expected both global and adhoc types, got %v", types)
	}
}

// ==================== ALL TASKS FOR STUDENT ====================

func TestAllTasksForStudent(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a global item
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Global Task", Priority: "medium", Recurrence: "daily", Active: true,
	})
	// Create a student-specific item
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Student Task", Priority: "low", Recurrence: "none", Active: true,
	})

	req := reqWithSession("GET", "/api/dashboard/all-tasks?student_id=S001", "", app, "user", "student", "S001")
	w := doReq(app.HandleDashboardAllTasks, req)
	resp := mustDecode(t, w)

	globalItems, _ := resp["global_items"].([]any)
	studentItems, _ := resp["student_items"].([]any)

	if len(globalItems) != 1 {
		t.Errorf("expected 1 global item, got %d", len(globalItems))
	}
	if len(studentItems) != 1 {
		t.Errorf("expected 1 student item, got %d", len(studentItems))
	}
}

// ==================== REQUIRES SIGNOFF ====================

func TestRequiresSignoff_Defaults(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher item — defaults to signoff=true when explicitly set
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Teacher Task","priority":"medium","recurrence":"none","requires_signoff":true}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	mustDecode(t, w)

	// Student item — no signoff
	req = reqWithSession("POST", "/api/tracker/student-items",
		`{"name":"My Notes","priority":"low","recurrence":"none","requires_signoff":false}`,
		app, "user", "student", "S001")
	w = doReq(app.HandleStudentTrackerItems, req)
	mustDecode(t, w)

	// Check both items
	items, _ := database.ListStudentTrackerItems(app.DB, "S001")
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	for _, it := range items {
		switch it.Name {
		case "Teacher Task":
			if !it.RequiresSignoff {
				t.Error("Teacher Task should have requires_signoff=true")
			}
		case "My Notes":
			if it.RequiresSignoff {
				t.Error("My Notes should have requires_signoff=false")
			}
		}
	}
}

// ==================== ADMIN CAN DO ANYTHING ====================

func TestAdmin_CanDeleteAnyItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates an item
	req := reqWithSession("POST", "/api/tracker/student-items",
		`{"student_id":"S001","name":"Teacher's Item","priority":"medium","recurrence":"none"}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItems, req)
	resp := mustDecode(t, w)
	itemID := resp["id"].(float64)

	// Admin can delete it
	req = reqWithSession("POST", "/api/tracker/student-items/delete",
		`{"id":`+jsonNum(itemID)+`}`,
		app, "admin", "", "admin")
	w = doReq(app.HandleStudentTrackerItemDelete, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("admin delete failed: %v", resp)
	}
}

// ==================== HELPERS ====================

func jsonNum(f float64) string {
	return strings.TrimRight(strings.TrimRight(
		strings.Replace(
			json.Number(strings.TrimRight(strings.TrimRight(
				func() string { b, _ := json.Marshal(f); return string(b) }(),
				"0"), ".")).String(),
			"e+", "e", 1),
		"0"), ".")
}
