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
		Name: "Daily Math Quiz", Priority: "high", Recurrence: "daily", Category: "Math", Type: models.TaskTypeTodo, Active: true,
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
	if err := database.DeleteTrackerItem(app.DB, int(id), "admin"); err != nil {
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
	// Verify audit fields
	if withDeleted[0].DeletedAt == "" {
		t.Error("expected deleted_at to be set after soft-delete")
	}
	if withDeleted[0].DeletedBy != "admin" {
		t.Errorf("expected deleted_by='admin', got %q", withDeleted[0].DeletedBy)
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
	// requires_signoff=true in the API request maps to type=todo
	if items[0]["type"] != "todo" {
		t.Errorf("expected type=todo, got %q", items[0]["type"])
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
	if items[0]["type"] != "task" {
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
		if items[0]["type"] != "todo" {
			t.Errorf("student %s: expected requires_signoff=true", sid)
		}
	}

	// Original library item should be removed after assignment (template consumed)
	req = reqWithSession("GET", "/api/dashboard/teacher-items", "", app, "user", "teacher", "T001")
	w = doReq(app.HandleDashboardTeacherItems, req)
	items := mustDecodeArray(t, w)
	// Should have 2: only the assigned copies, template is soft-deleted
	if len(items) != 2 {
		t.Errorf("expected 2 items (assigned copies only), got %d", len(items))
	}
	for _, it := range items {
		if it["student_id"] == "" {
			t.Error("expected no unassigned library items after assignment")
		}
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
		Name: "Daily Warmup", Priority: "medium", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
	})
	// Create a one-time student item
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "One-time Essay", Priority: "high", Recurrence: "none", Active: true, Type: models.TaskTypeTodo,
	})

	// Get due items for S001 today
	req := reqWithSession("GET", "/api/tracker/due?student_id=S001", "", app, "user", "student", "S001")
	w := doReq(app.HandleTrackerDue, req)
	var items []map[string]any
	json.NewDecoder(w.Body).Decode(&items)

	if len(items) != 2 {
		t.Fatalf("expected 2 due items (1 global + 1 personal), got %d", len(items))
	}

	types := map[string]bool{}
	for _, it := range items {
		types[it["item_type"].(string)] = true
	}
	if !types["global"] || !types["personal"] {
		t.Errorf("expected both global and personal types, got %v", types)
	}
}

// ==================== ALL TASKS FOR STUDENT ====================

func TestAllTasksForStudent(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a global item
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Global Task", Priority: "medium", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
	})
	// Create a student-specific item
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Student Task", Priority: "low", Recurrence: "none", Type: models.TaskTypeTodo, Active: true,
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
			if it.Type != models.TaskTypeTodo {
				t.Errorf("Teacher Task should have type=todo, got %s", it.Type)
			}
		case "My Notes":
			if it.Type != models.TaskTypeTask {
				t.Errorf("My Notes should have type=task, got %s", it.Type)
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

// ==================== PROGRESS STATS ====================

func TestProgress_ExpectedBased_DailyItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a daily global item active for a known 5-day range
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Daily Check", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-13", EndDate: "2026-04-17", Type: models.TaskTypeTodo, Active: true,
	})

	// Student S001 responds "done" on 3 of the 5 days
	for _, d := range []string{"2026-04-13", "2026-04-14", "2026-04-16"} {
		app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
			VALUES ('S001', 'Alice', 'global', 1, 'Daily Check', 'done', ?, datetime('now','localtime'))`, d)
	}

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-13", "2026-04-17")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	s := stats[0]
	if s.TotalItems != 5 {
		t.Errorf("expected 5 expected items (daily x 5 days), got %d", s.TotalItems)
	}
	if s.DoneCount != 3 {
		t.Errorf("expected 3 done, got %d", s.DoneCount)
	}
	if s.NotDone != 2 {
		t.Errorf("expected 2 missed, got %d", s.NotDone)
	}
	if s.Completion < 59 || s.Completion > 61 {
		t.Errorf("expected ~60%% completion, got %.1f%%", s.Completion)
	}
}

func TestProgress_ExpectedBased_WeeklyItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a weekly item spanning 2 weeks (Mon Apr 13 - Sun Apr 26 = 2 ISO weeks)
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Weekly Review", Priority: "medium", Recurrence: "weekly",
		StartDate: "2026-04-13", EndDate: "2026-04-26", Type: models.TaskTypeTodo, Active: true,
	})

	// Student responds in week 1 only
	app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
		VALUES ('S001', 'Alice', 'global', 1, 'Weekly Review', 'done', '2026-04-15', datetime('now','localtime'))`)

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-13", "2026-04-26")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	s := stats[0]
	if s.TotalItems != 2 {
		t.Errorf("expected 2 expected items (weekly x 2 weeks), got %d", s.TotalItems)
	}
	if s.DoneCount != 1 {
		t.Errorf("expected 1 done, got %d", s.DoneCount)
	}
	if s.Completion < 49 || s.Completion > 51 {
		t.Errorf("expected ~50%% completion, got %.1f%%", s.Completion)
	}
}

func TestProgress_ExpectedBased_MonthlyItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Monthly item spanning 2 months
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Monthly Report", Priority: "low", Recurrence: "monthly",
		StartDate: "2026-03-15", EndDate: "2026-04-30", Type: models.TaskTypeTodo, Active: true,
	})

	// Student responds in both months
	app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
		VALUES ('S001', 'Alice', 'global', 1, 'Monthly Report', 'done', '2026-03-20', datetime('now','localtime'))`)
	app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
		VALUES ('S001', 'Alice', 'global', 1, 'Monthly Report', 'done', '2026-04-10', datetime('now','localtime'))`)

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-03-15", "2026-04-30")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 2 {
		t.Errorf("expected 2 expected (monthly x 2 months), got %d", s.TotalItems)
	}
	if s.DoneCount != 2 {
		t.Errorf("expected 2 done, got %d", s.DoneCount)
	}
	if s.Completion < 99 {
		t.Errorf("expected 100%% completion, got %.1f%%", s.Completion)
	}
}

func TestProgress_ExpectedBased_OneTimeItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// One-time item with a date window
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Submit Application", Priority: "high", Recurrence: "none",
		StartDate: "2026-04-10", EndDate: "2026-04-20", Type: models.TaskTypeTodo, Active: true,
	})

	// Student does NOT respond — should have 0/1 completion
	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-10", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	s := stats[0]
	if s.TotalItems != 1 {
		t.Errorf("expected 1 expected (one-time), got %d", s.TotalItems)
	}
	if s.DoneCount != 0 {
		t.Errorf("expected 0 done, got %d", s.DoneCount)
	}
	if s.Completion != 0 {
		t.Errorf("expected 0%% completion, got %.1f%%", s.Completion)
	}
}

func TestProgress_NoDateConstraints(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Daily item with no start/end dates — always active
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Open-ended Daily", Priority: "medium", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
	})

	// Query a 3-day range
	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 3 {
		t.Errorf("expected 3 expected (daily x 3 days, no date constraint), got %d", s.TotalItems)
	}
	if s.DoneCount != 0 {
		t.Errorf("expected 0 done, got %d", s.DoneCount)
	}
}

func TestProgress_StartDateOnly(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Daily item starting Apr 19, querying Apr 18-20 — only active on 19 and 20
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Late Start", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-19", Type: models.TaskTypeTodo, Active: true,
	})

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 2 {
		t.Errorf("expected 2 expected (daily, start Apr 19, 2 of 3 days), got %d", s.TotalItems)
	}
}

func TestProgress_EndDateOnly(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Daily item ending Apr 19, querying Apr 18-20 — active on 18 and 19
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Early End", Priority: "medium", Recurrence: "daily",
		EndDate: "2026-04-19", Type: models.TaskTypeTodo, Active: true,
	})

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 2 {
		t.Errorf("expected 2 expected (daily, end Apr 19, 2 of 3 days), got %d", s.TotalItems)
	}
}

func TestProgress_StudentSpecificItems(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Global daily item
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Global Task", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-18", EndDate: "2026-04-20", Type: models.TaskTypeTodo, Active: true,
	})
	// Student-specific daily item for S001 only
	database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Personal Task", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-18", EndDate: "2026-04-20", Type: models.TaskTypeTodo, Active: true,
	})

	// S001 should have 6 expected (3 global + 3 student)
	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 6 {
		t.Errorf("expected 6 expected (3 global + 3 student), got %d", s.TotalItems)
	}

	// S002 should have 3 expected (3 global only, no student items)
	stats, err = database.GetProgressStats(app.DB, []string{"S002"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s = stats[0]
	if s.TotalItems != 3 {
		t.Errorf("expected 3 expected (global only for S002), got %d", s.TotalItems)
	}
}

func TestProgress_MultipleStudents(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// One global daily item, 2-day range
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Daily", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-18", EndDate: "2026-04-19", Type: models.TaskTypeTodo, Active: true,
	})

	// S001 does 2/2, S002 does 1/2
	app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
		VALUES ('S001', 'Alice', 'global', 1, 'Daily', 'done', '2026-04-18', datetime('now','localtime'))`)
	app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
		VALUES ('S001', 'Alice', 'global', 1, 'Daily', 'done', '2026-04-19', datetime('now','localtime'))`)
	app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
		VALUES ('S002', 'Bob', 'global', 1, 'Daily', 'done', '2026-04-18', datetime('now','localtime'))`)

	stats, err := database.GetProgressStats(app.DB, []string{"S001", "S002"}, "2026-04-18", "2026-04-19")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}

	statsMap := map[string]models.ProgressStats{}
	for _, s := range stats {
		statsMap[s.StudentID] = s
	}

	s1 := statsMap["S001"]
	if s1.TotalItems != 2 || s1.DoneCount != 2 || s1.Completion < 99 {
		t.Errorf("S001: expected 2/2 100%%, got %d/%d %.0f%%", s1.DoneCount, s1.TotalItems, s1.Completion)
	}
	s2 := statsMap["S002"]
	if s2.TotalItems != 2 || s2.DoneCount != 1 || s2.Completion < 49 || s2.Completion > 51 {
		t.Errorf("S002: expected 1/2 50%%, got %d/%d %.0f%%", s2.DoneCount, s2.TotalItems, s2.Completion)
	}
}

func TestProgress_DoneCappedAtExpected(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// One-time item
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "One Shot", Priority: "medium", Recurrence: "none",
		StartDate: "2026-04-18", EndDate: "2026-04-20", Type: models.TaskTypeTodo, Active: true,
	})

	// Student responds done multiple times (shouldn't exceed 100%)
	for _, d := range []string{"2026-04-18", "2026-04-19", "2026-04-20"} {
		app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
			VALUES ('S001', 'Alice', 'global', 1, 'One Shot', 'done', ?, datetime('now','localtime'))`, d)
	}

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 1 {
		t.Errorf("expected 1 expected (one-time), got %d", s.TotalItems)
	}
	if s.DoneCount != 1 {
		t.Errorf("expected done capped at 1, got %d", s.DoneCount)
	}
	if s.Completion > 101 {
		t.Errorf("completion should not exceed 100%%, got %.1f%%", s.Completion)
	}
}

func TestProgress_InactiveItemsExcluded(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Active item
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Active Task", Priority: "medium", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
	})
	// Inactive item — should not count toward expected
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Inactive Task", Priority: "medium", Recurrence: "daily", Type: models.TaskTypeTodo, Active: false,
	})

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 3 {
		t.Errorf("expected 3 expected (only active daily x 3 days), got %d", s.TotalItems)
	}
}

func TestProgress_OutsideDateWindow(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Item with start/end outside the query range
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Past Item", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-03-01", EndDate: "2026-03-10", Type: models.TaskTypeTodo, Active: true,
	})

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	if s.TotalItems != 0 {
		t.Errorf("expected 0 expected (item window doesn't overlap query range), got %d", s.TotalItems)
	}
}

// ==================== PROGRESS API E2E ====================

func TestProgress_API_EndToEnd(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// 1. Admin creates a daily global item for a specific date range
	req := reqWithSession("POST", "/api/v1/tracker/items",
		`{"name":"Homework Check","priority":"medium","recurrence":"daily","start_date":"2026-04-13","end_date":"2026-04-17","active":true}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create item failed: %v", resp)
	}

	// 2. Verify item was created with end_date (not due_date)
	req = reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["end_date"] != "2026-04-17" {
		t.Errorf("expected end_date=2026-04-17, got %v", items[0]["end_date"])
	}
	if _, hasDue := items[0]["due_date"]; hasDue {
		t.Error("should not have due_date field, should be end_date")
	}

	// 3. Student checks in and responds to the daily item for 3 of 5 days
	for _, d := range []string{"2026-04-13", "2026-04-14", "2026-04-16"} {
		app.DB.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, response_date, responded_at)
			VALUES ('S001', 'Alice', 'global', ?, 'Homework Check', 'done', ?, datetime('now','localtime'))`,
			int(items[0]["id"].(float64)), d)
	}

	// 4. Query progress API
	req = reqWithSession("GET", "/api/dashboard/progress?student_id=S001&start_date=2026-04-13&end_date=2026-04-17",
		"", app, "user", "student", "S001")
	w = doReq(app.HandleTrackerProgress, req)
	var progressStats []map[string]any
	json.NewDecoder(w.Body).Decode(&progressStats)

	if len(progressStats) != 1 {
		t.Fatalf("expected 1 progress entry, got %d", len(progressStats))
	}
	ps := progressStats[0]
	totalItems := int(ps["total_items"].(float64))
	doneCount := int(ps["done_count"].(float64))
	notDone := int(ps["not_done_count"].(float64))
	completionPct := ps["completion_pct"].(float64)

	if totalItems != 5 {
		t.Errorf("expected total_items=5 (daily x 5), got %d", totalItems)
	}
	if doneCount != 3 {
		t.Errorf("expected done_count=3, got %d", doneCount)
	}
	if notDone != 2 {
		t.Errorf("expected not_done_count=2, got %d", notDone)
	}
	if completionPct < 59 || completionPct > 61 {
		t.Errorf("expected ~60%% completion, got %.1f%%", completionPct)
	}
}

func TestProgress_API_EndDateField(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create item via API with end_date
	req := reqWithSession("POST", "/api/v1/tracker/items",
		`{"name":"Test Item","priority":"low","recurrence":"weekly","end_date":"2026-05-01"}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create failed: %v", resp)
	}

	// Verify round-trip
	req = reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	items := mustDecodeArray(t, w)
	if items[0]["end_date"] != "2026-05-01" {
		t.Errorf("expected end_date=2026-05-01, got %v", items[0]["end_date"])
	}

	// Verify no due_date key exists
	if _, ok := items[0]["due_date"]; ok {
		t.Error("API should return end_date, not due_date")
	}
}

// ==================== REQUIRES_SIGNOFF TESTS ====================

func TestGlobalItem_RequiresSignoff_BlocksCheckout(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()
	app.PinMode = "off"
	app.SetRequirePIN(false)

	// Create a global item with requires_signoff=true
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Must Sign Off", Priority: "high", Recurrence: "daily",
		Type: models.TaskTypeTodo, Active: true,
	})

	// Check in student S001
	app.DB.Exec(`INSERT INTO attendance (student_name, student_id, check_in_time) VALUES ('Alice', 'S001', datetime('now','localtime'))`)

	// Attempt checkout — should be blocked
	pending, _ := database.PendingSignoffItems(app.DB, "S001")
	if len(pending) == 0 {
		t.Fatal("expected global item with requires_signoff=true to block checkout")
	}
	if pending[0].ItemType != "global" {
		t.Errorf("expected item_type=global, got %s", pending[0].ItemType)
	}
}

func TestGlobalItem_NoSignoff_DoesNotBlockCheckout(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a global item with requires_signoff=false
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Informational", Priority: "medium", Recurrence: "daily",
		Type: models.TaskTypeTask, Active: true,
	})

	pending, _ := database.PendingSignoffItems(app.DB, "S001")
	if len(pending) != 0 {
		t.Errorf("expected no pending items for requires_signoff=false, got %d", len(pending))
	}
}

func TestProgress_ExcludesNonSignoffItems(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create global item with requires_signoff=false — should NOT count
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Info Only", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-18", EndDate: "2026-04-20",
		Type: models.TaskTypeTask, Active: true,
	})
	// Create global item with requires_signoff=true — should count
	database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Required", Priority: "medium", Recurrence: "daily",
		StartDate: "2026-04-18", EndDate: "2026-04-20",
		Type: models.TaskTypeTodo, Active: true,
	})

	stats, err := database.GetProgressStats(app.DB, []string{"S001"}, "2026-04-18", "2026-04-20")
	if err != nil {
		t.Fatalf("GetProgressStats: %v", err)
	}
	s := stats[0]
	// Only the "Required" item should count: 3 days of daily
	if s.TotalItems != 3 {
		t.Errorf("expected 3 expected items (only signoff items), got %d", s.TotalItems)
	}
}

func TestCompleteStudentItem_CreatesTrackerResponse(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a student-specific item
	id, _ := database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Finish Essay", Priority: "high",
		Recurrence: "none", Type: models.TaskTypeTodo, Active: true,
	})

	// Complete it
	err := database.CompleteStudentTrackerItem(app.DB, int(id), "teacher1")
	if err != nil {
		t.Fatalf("CompleteStudentTrackerItem: %v", err)
	}

	// Verify tracker_responses row was created
	var count int
	app.DB.QueryRow(`SELECT COUNT(*) FROM tracker_responses WHERE item_type='personal' AND item_id=? AND status='done'`, id).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 tracker_response row after completion, got %d", count)
	}
}

func TestUncompleteStudentItem_RemovesTrackerResponse(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	id, _ := database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		StudentID: "S001", Name: "Finish Essay", Priority: "high",
		Recurrence: "none", Type: models.TaskTypeTodo, Active: true,
	})

	// Complete then uncomplete
	database.CompleteStudentTrackerItem(app.DB, int(id), "teacher1")
	err := database.UncompleteStudentTrackerItem(app.DB, int(id))
	if err != nil {
		t.Fatalf("UncompleteStudentTrackerItem: %v", err)
	}

	// Verify tracker_responses row was removed
	var count int
	app.DB.QueryRow(`SELECT COUNT(*) FROM tracker_responses WHERE item_type='personal' AND item_id=? AND status='done' AND attendance_id=0`, id).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 tracker_response rows after uncomplete, got %d", count)
	}
}

func TestBulkAssign_RequiresSignoff(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Bulk assign with requires_signoff=false
	req := reqWithSession("POST", "/api/dashboard/bulk-assign",
		`{"student_ids":["S001","S002"],"name":"Optional Task","requires_signoff":false}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleTrackerBulkAssign, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("bulk assign failed: %v", resp)
	}

	// Verify items were created with type=task (requires_signoff=false maps to task)
	var itemType string
	app.DB.QueryRow("SELECT type FROM task_items WHERE student_id='S001' AND name='Optional Task'").Scan(&itemType)
	if itemType != "task" {
		t.Errorf("expected type=task, got %s", itemType)
	}

	// Bulk assign without requires_signoff (should default to todo)
	req = reqWithSession("POST", "/api/dashboard/bulk-assign",
		`{"student_ids":["S001"],"name":"Required Task"}`,
		app, "admin", "", "admin")
	w = doReq(app.HandleTrackerBulkAssign, req)
	resp = mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("bulk assign failed: %v", resp)
	}

	app.DB.QueryRow("SELECT type FROM task_items WHERE student_id='S001' AND name='Required Task'").Scan(&itemType)
	if itemType != "todo" {
		t.Errorf("expected type=todo (default), got %s", itemType)
	}
}

func TestGlobalItem_RequiresSignoff_RoundTrip(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create with requires_signoff=false via API
	req := reqWithSession("POST", "/api/v1/tracker/items",
		`{"name":"Info Item","priority":"low","recurrence":"daily","requires_signoff":false}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleTrackerItems, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("create failed: %v", resp)
	}

	// Read back and verify
	req = reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	items := mustDecodeArray(t, w)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["type"] != "task" {
		t.Errorf("expected requires_signoff=false, got %v", items[0]["requires_signoff"])
	}

	// Create with requires_signoff=true
	req = reqWithSession("POST", "/api/v1/tracker/items",
		`{"name":"Required Item","priority":"high","recurrence":"daily","requires_signoff":true}`,
		app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)

	req = reqWithSession("GET", "/api/v1/tracker/items", "", app, "admin", "", "admin")
	w = doReq(app.HandleTrackerItems, req)
	items = mustDecodeArray(t, w)
	found := false
	for _, it := range items {
		if it["name"] == "Required Item" && it["type"] == "todo" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find Required Item with requires_signoff=true")
	}
}

// ==================== SOFT-DELETE AUDIT ====================

func TestSoftDeleteAudit_Entity(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a student
	app.DB.Exec("INSERT INTO students (id, first_name, last_name, active) VALUES ('AUDIT1', 'Audit', 'Test', 1)")

	// Delete via API handler
	req := reqWithSession("POST", "/api/v1/data",
		`{"action":"delete","type":"students","id":"AUDIT1"}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleDataCRUD, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("delete failed: %v", resp)
	}

	// Verify audit columns in DB
	var deletedAt, deletedBy string
	err := app.DB.QueryRow("SELECT COALESCE(deleted_at,''), COALESCE(deleted_by,'') FROM students WHERE id = 'AUDIT1'").Scan(&deletedAt, &deletedBy)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if deletedAt == "" {
		t.Error("expected deleted_at to be set")
	}
	if deletedBy != "admin" {
		t.Errorf("expected deleted_by='admin', got %q", deletedBy)
	}
}

func TestSoftDeleteAudit_EntityAPI(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a student
	app.DB.Exec("INSERT INTO students (id, first_name, last_name, active) VALUES ('AUDIT2', 'Audit2', 'Test', 1)")

	// Delete via API handler
	req := reqWithSession("POST", "/api/v1/data",
		`{"action":"delete","type":"students","id":"AUDIT2"}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleDataCRUD, req)
	mustDecode(t, w)

	// Get directory with include_deleted via handler
	req = reqWithSession("GET", "/api/v1/directory?include_deleted=1", "", app, "admin", "", "admin")
	w = doReq(app.HandleDirectoryAPI, req)
	resp := mustDecode(t, w)

	students, _ := resp["students"].([]any)
	var found map[string]any
	for _, s := range students {
		m, _ := s.(map[string]any)
		if m["id"] == "AUDIT2" {
			found = m
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find deleted student AUDIT2 with include_deleted=1")
	}
	if found["deleted"] != true {
		t.Error("expected deleted=true")
	}
	if found["deleted_at"] == nil || found["deleted_at"] == "" {
		t.Error("expected deleted_at to be set in API response")
	}
	if found["deleted_by"] == nil || found["deleted_by"] == "" {
		t.Error("expected deleted_by to be set in API response")
	}
}

func TestSoftDeleteAudit_TaskItem(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Create a global task
	id, err := database.SaveTrackerItem(app.DB, models.TrackerItem{
		Name: "Audit Task", Priority: "medium", Recurrence: "daily", Type: models.TaskTypeTodo, Active: true,
	})
	if err != nil {
		t.Fatalf("SaveTrackerItem: %v", err)
	}

	// Delete via handler
	req := reqWithSession("POST", "/api/v1/tracker/items/delete",
		`{"id":`+jsonNum(float64(id))+`}`,
		app, "admin", "", "admin")
	w := doReq(app.HandleTrackerItemDelete, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("delete failed: %v", resp)
	}

	// List with include_deleted
	items, _ := database.ListTrackerItems(app.DB, true)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].DeletedAt == "" {
		t.Error("expected deleted_at to be set")
	}
	if items[0].DeletedBy == "" {
		t.Error("expected deleted_by to be set")
	}
}

func TestSoftDeleteAudit_StudentTaskByTeacher(t *testing.T) {
	app, cleanup := setupTrackerTest(t)
	defer cleanup()

	// Teacher creates a personal task
	id, err := database.SaveStudentTrackerItem(app.DB, models.StudentTrackerItem{
		Scope: models.ScopePersonal, StudentID: "S001", Name: "Teacher Task",
		Priority: "low", Recurrence: "none", CreatedBy: "T001", OwnerType: "teacher",
		Type: models.TaskTypeTask, Active: true,
	})
	if err != nil {
		t.Fatalf("SaveStudentTrackerItem: %v", err)
	}

	// Teacher deletes their own task
	req := reqWithSession("POST", "/api/tracker/student-items/delete",
		`{"id":`+jsonNum(float64(id))+`}`,
		app, "user", "teacher", "T001")
	w := doReq(app.HandleStudentTrackerItemDelete, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("delete failed: %v", resp)
	}

	// Verify deleted_by is the teacher
	var deletedBy string
	app.DB.QueryRow("SELECT COALESCE(deleted_by,'') FROM task_items WHERE id = ?", id).Scan(&deletedBy)
	if deletedBy != "T001" {
		t.Errorf("expected deleted_by='T001', got %q", deletedBy)
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
