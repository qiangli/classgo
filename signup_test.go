package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/handlers"
	"classgo/memos/lib/profile"
	memosstore "classgo/memos/store"
	memossqlite "classgo/memos/store/db/sqlite"
)

// setupSignupTest creates a test app with Memos store (needed for signup/login).
func setupSignupTest(t *testing.T) (*handlers.App, func()) {
	t.Helper()

	app, baseCleanup := setupTest(t)
	app.Sessions = auth.NewSessionStore()

	// Set up Memos store with a temp DB
	memosDBFile, err := os.CreateTemp("", "memos-test-*.db")
	if err != nil {
		baseCleanup()
		t.Fatal(err)
	}
	memosDBFile.Close()
	memosDBPath := memosDBFile.Name()

	memosProfile := &profile.Profile{
		DSN:     memosDBPath,
		Driver:  "sqlite",
		Version: "0.27.1",
	}
	memosDriver, err := memossqlite.NewDB(memosProfile)
	if err != nil {
		os.Remove(memosDBPath)
		baseCleanup()
		t.Fatal(err)
	}
	memosStoreInst := memosstore.New(memosDriver, memosProfile)
	if err := memosStoreInst.Migrate(context.Background()); err != nil {
		os.Remove(memosDBPath)
		baseCleanup()
		t.Fatal(err)
	}
	app.MemosStore = memosStoreInst

	// Insert test students with full names for signup tests
	app.DB.Exec("INSERT OR REPLACE INTO students (id, first_name, last_name, active) VALUES ('S010', 'Grace', 'Lee', 1)")
	app.DB.Exec("INSERT OR REPLACE INTO students (id, first_name, last_name, active) VALUES ('S011', 'Henry', 'Kim', 1)")
	app.DB.Exec("INSERT OR REPLACE INTO parents (id, first_name, last_name) VALUES ('P010', 'Jennifer', 'Lee')")

	return app, func() {
		os.Remove(memosDBPath)
		baseCleanup()
	}
}

func signupJSON(app *handlers.App, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.HandleLoginAPI(w, req)
	return w
}

// ==================== SIGNUP ====================

func TestSignup_Success(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	w := signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)
	resp := mustDecode(t, w)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %v", w.Code, resp)
	}
	if resp["ok"] != true {
		t.Fatalf("signup failed: %v", resp)
	}
	if resp["redirect"] != "/profile" {
		t.Errorf("expected redirect to /profile, got %v", resp["redirect"])
	}
	if resp["role"] != "user" {
		t.Errorf("expected role=user, got %v", resp["role"])
	}

	// Verify session cookie is set
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == auth.SessionCookie && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected session cookie to be set after signup")
	}
}

func TestSignup_CaseInsensitive(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	w := signupJSON(app, `{"action":"signup","first_name":"grace","last_name":"lee","password":"pass1234"}`)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("case-insensitive signup should succeed: %v", resp)
	}
}

func TestSignup_ParentCanSignup(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	w := signupJSON(app, `{"action":"signup","first_name":"Jennifer","last_name":"Lee","password":"pass1234"}`)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("parent signup failed: %v", resp)
	}
}

func TestSignup_DuplicateAccount(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// First signup succeeds
	signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)

	// Second signup should fail
	w := signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass5678"}`)
	resp := mustDecode(t, w)

	if resp["ok"] == true {
		t.Fatal("duplicate signup should fail")
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(strings.ToLower(errMsg), "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", errMsg)
	}
}

func TestSignup_StudentNotFound(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	w := signupJSON(app, `{"action":"signup","first_name":"Nobody","last_name":"Here","password":"pass1234"}`)
	resp := mustDecode(t, w)

	if resp["ok"] == true {
		t.Fatal("signup with unknown name should fail")
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(strings.ToLower(errMsg), "no student found") {
		t.Errorf("expected 'no student found' error, got: %s", errMsg)
	}
}

func TestSignup_MissingFields(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Missing last name
	w := signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"","password":"pass1234"}`)
	if w.Code == 200 {
		resp := mustDecode(t, w)
		if resp["ok"] == true {
			t.Error("signup with empty last name should fail")
		}
	}

	// Missing password
	w = signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":""}`)
	if w.Code == 200 {
		resp := mustDecode(t, w)
		if resp["ok"] == true {
			t.Error("signup with empty password should fail")
		}
	}

	// Short password
	w = signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"abc"}`)
	resp := mustDecode(t, w)
	if resp["ok"] == true {
		t.Error("signup with short password should fail")
	}
}

// ==================== LOGIN ====================

func TestLogin_Success(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Signup first
	signupJSON(app, `{"action":"signup","first_name":"Henry","last_name":"Kim","password":"mypassword"}`)

	// Login
	w := signupJSON(app, `{"action":"login","entity_id":"S011","password":"mypassword"}`)
	resp := mustDecode(t, w)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %v", w.Code, resp)
	}
	if resp["ok"] != true {
		t.Fatalf("login failed: %v", resp)
	}
	if resp["redirect"] != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %v", resp["redirect"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	signupJSON(app, `{"action":"signup","first_name":"Henry","last_name":"Kim","password":"mypassword"}`)

	w := signupJSON(app, `{"action":"login","entity_id":"S011","password":"wrongpass"}`)
	resp := mustDecode(t, w)

	if resp["ok"] == true {
		t.Fatal("login with wrong password should fail")
	}
}

func TestLogin_NoAccount(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	w := signupJSON(app, `{"action":"login","entity_id":"S010","password":"anything"}`)
	resp := mustDecode(t, w)

	if resp["ok"] == true {
		t.Fatal("login without account should fail")
	}
}

func TestLogin_CheckHasPassword(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Before signup: no password
	w := signupJSON(app, `{"action":"check","entity_id":"S010"}`)
	resp := mustDecode(t, w)
	if resp["has_password"] != false {
		t.Error("expected has_password=false before signup")
	}

	// After signup: has password
	signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)

	w = signupJSON(app, `{"action":"check","entity_id":"S010"}`)
	resp = mustDecode(t, w)
	if resp["has_password"] != true {
		t.Error("expected has_password=true after signup")
	}
}

// ==================== SIGNUP THEN CHECKIN/CHECKOUT ====================

func TestSignupThenCheckinCheckout(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Signup
	w := signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("signup failed: %v", resp)
	}

	// Check in (check-in doesn't require auth session, just PIN)
	w = postJSON(app.HandleCheckIn, `{"student_name":"Grace Lee","pin":"1234","device_type":"mobile"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-in after signup failed: %v", resp)
	}

	// Verify checked in
	w = getJSON(app.HandleStatus, "/api/status?student_name=Grace+Lee")
	resp = decodeResp(t, w)
	if resp["checked_in"] != true {
		t.Error("expected checked_in=true")
	}

	// Check out
	w = postJSON(app.HandleCheckOut, `{"student_name":"Grace Lee","pin":"1234"}`)
	resp = decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("check-out after signup failed: %v", resp)
	}

	// Verify checked out
	w = getJSON(app.HandleStatus, "/api/status?student_name=Grace+Lee")
	resp = decodeResp(t, w)
	if resp["checked_out"] != true {
		t.Error("expected checked_out=true")
	}
}

// ==================== PROFILE ACCESS AFTER SIGNUP ====================

func TestSignup_ProfileAccessControl(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Signup as student
	signupJSON(app, `{"action":"signup","first_name":"Grace","last_name":"Lee","password":"pass1234"}`)

	// Access own profile via API
	req := reqWithSession(http.MethodGet, "/api/v1/user/profile?student_id=S010", "", app, "user", "student", "S010")
	w := doReq(app.HandleUserProfile, req)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("student should access own profile: %v", resp)
	}
	if resp["is_empty_profile"] != true {
		t.Error("new signup should have empty profile")
	}

	// Student always resolves to own profile regardless of student_id param
	// (resolveStudentID returns sess.EntityID for students, ignoring query param)
	req = reqWithSession(http.MethodGet, "/api/v1/user/profile?student_id=S011", "", app, "user", "student", "S010")
	w = doReq(app.HandleUserProfile, req)
	resp = mustDecode(t, w)

	student, _ := resp["student"].(map[string]any)
	if student != nil && student["id"] != "S010" {
		t.Error("student should always resolve to own profile, not another student's")
	}
}

func TestSignup_ParentAccessChildProfile(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Link student to parent
	app.DB.Exec("UPDATE students SET parent_id = 'P010' WHERE id = 'S010'")

	// Signup as parent
	signupJSON(app, `{"action":"signup","first_name":"Jennifer","last_name":"Lee","password":"pass1234"}`)

	// Access child's profile
	req := reqWithSession(http.MethodGet, "/api/v1/user/profile?student_id=S010", "", app, "user", "parent", "P010")
	w := doReq(app.HandleUserProfile, req)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("parent should access child's profile: %v", resp)
	}

	// Cannot access non-child's profile
	req = reqWithSession(http.MethodGet, "/api/v1/user/profile?student_id=S011", "", app, "user", "parent", "P010")
	w = doReq(app.HandleUserProfile, req)
	resp = mustDecode(t, w)

	if resp["ok"] == true {
		t.Error("parent should NOT access non-child's profile")
	}
}

// ==================== PROFILE SAVE + DRAFT STATUS ====================

func TestProfileSave_DraftStatus(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Save profile as student → should set draft status
	body := `{"student":{"id":"S010","first_name":"Grace","last_name":"Lee","grade":"10","school":"Lincoln High","dob":"2010-05-15"},"parent":{},"tracker_values":{}}`
	req := reqWithSession(http.MethodPost, "/api/v1/user/profile", body, app, "user", "student", "S010")
	w := doReq(app.HandleUserProfile, req)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("profile save failed: %v", resp)
	}

	// Verify draft status
	var status string
	app.DB.QueryRow("SELECT profile_status FROM students WHERE id = 'S010'").Scan(&status)
	if status != "draft" {
		t.Errorf("expected profile_status='draft', got %q", status)
	}

	// Verify fields saved
	var dob, school string
	app.DB.QueryRow("SELECT dob, school FROM students WHERE id = 'S010'").Scan(&dob, &school)
	if dob != "2010-05-15" {
		t.Errorf("expected dob='2010-05-15', got %q", dob)
	}
	if school != "Lincoln High" {
		t.Errorf("expected school='Lincoln High', got %q", school)
	}
}

func TestProfileFinalize_AdminOnly(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Set draft status
	app.DB.Exec("UPDATE students SET profile_status = 'draft' WHERE id = 'S010'")

	// Admin finalizes
	body := `{"student":{"id":"S010","first_name":"Grace","last_name":"Lee"},"parent":{},"finalize":true}`
	req := reqWithSession(http.MethodPost, "/api/v1/student/profile", body, app, "admin", "", "")
	w := doReq(app.HandleStudentProfile, req)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("finalize failed: %v", resp)
	}

	var status string
	app.DB.QueryRow("SELECT profile_status FROM students WHERE id = 'S010'").Scan(&status)
	if status != "final" {
		t.Errorf("expected profile_status='final', got %q", status)
	}
}

// ==================== AUTO-ASSIGN TASKS FROM PROFILE GAPS ====================

func TestAutoAssignProfileTasks(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	// Seed tracker items
	database.SeedSampleData(app.DB)

	// Save profile with grade 10, no tracker values → should auto-assign tasks
	body := `{"student":{"id":"S010","first_name":"Grace","last_name":"Lee","grade":"10","school":"Lincoln High"},"parent":{},"tracker_values":{}}`
	req := reqWithSession(http.MethodPost, "/api/v1/user/profile", body, app, "user", "student", "S010")
	w := doReq(app.HandleUserProfile, req)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("profile save failed: %v", resp)
	}

	// Check auto-assigned items
	var count int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND created_by = 'system'").Scan(&count)
	if count == 0 {
		t.Fatal("expected auto-assigned tracker items for profile gaps")
	}

	// Verify grade-aware filtering: PSAT 8/9 should NOT be assigned (grade 10)
	var psat89Count int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND name LIKE '%PSAT 8/9%'").Scan(&psat89Count)
	if psat89Count > 0 {
		t.Error("PSAT 8/9 should not be assigned to grade 10 student")
	}

	// PSAT 10 SHOULD be assigned
	var psat10Count int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND name LIKE '%PSAT 10%'").Scan(&psat10Count)
	if psat10Count == 0 {
		t.Error("PSAT 10 should be assigned to grade 10 student")
	}

	// PSAT 11 should NOT be assigned (grade 10 < 11)
	var psat11Count int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND name LIKE '%PSAT 11%'").Scan(&psat11Count)
	if psat11Count > 0 {
		t.Error("PSAT 11 should not be assigned to grade 10 student")
	}

	// GPA items SHOULD be assigned
	var gpaCount int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND category = 'GPA'").Scan(&gpaCount)
	if gpaCount != 2 {
		t.Errorf("expected 2 GPA items assigned, got %d", gpaCount)
	}
}

func TestAutoAssign_SkipsExistingValues(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	database.SeedSampleData(app.DB)

	// Get the GPA tracker item ID
	var gpaItemID int
	app.DB.QueryRow("SELECT id FROM tracker_items WHERE name = 'Weighted GPA Update' AND deleted = 0").Scan(&gpaItemID)
	if gpaItemID == 0 {
		t.Fatal("GPA tracker item not found")
	}

	// Save profile WITH a GPA value
	body := `{"student":{"id":"S010","first_name":"Grace","last_name":"Lee","grade":"10","school":"Lincoln High"},"parent":{},"tracker_values":{"` +
		jsonInt(gpaItemID) + `":"3.85"}}`
	req := reqWithSession(http.MethodPost, "/api/v1/user/profile", body, app, "user", "student", "S010")
	w := doReq(app.HandleUserProfile, req)
	resp := mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("profile save failed: %v", resp)
	}

	// Weighted GPA should NOT be auto-assigned (has value)
	var wgpaCount int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND name = 'Weighted GPA Update' AND created_by = 'system'").Scan(&wgpaCount)
	if wgpaCount > 0 {
		t.Error("Weighted GPA should not be auto-assigned when value was provided")
	}

	// Unweighted GPA SHOULD be auto-assigned (no value)
	var uwgpaCount int
	app.DB.QueryRow("SELECT COUNT(*) FROM student_tracker_items WHERE student_id = 'S010' AND name = 'Unweighted GPA Update' AND created_by = 'system'").Scan(&uwgpaCount)
	if uwgpaCount == 0 {
		t.Error("Unweighted GPA should be auto-assigned when no value provided")
	}
}

// ==================== TRACKER VALUES SAVE/LOAD ====================

func TestProfileTrackerValues_SaveAndLoad(t *testing.T) {
	app, cleanup := setupSignupTest(t)
	defer cleanup()

	database.SeedSampleData(app.DB)

	// Get tracker item IDs
	var gpaID, satID int
	app.DB.QueryRow("SELECT id FROM tracker_items WHERE name = 'Weighted GPA Update' AND deleted = 0").Scan(&gpaID)
	app.DB.QueryRow("SELECT id FROM tracker_items WHERE name = 'SAT Score (1st Attempt)' AND deleted = 0").Scan(&satID)

	// Save tracker values via profile
	trackerJSON := `"` + jsonInt(gpaID) + `":"3.95","` + jsonInt(satID) + `":"E:750 M:780 T:1530"`
	body := `{"student":{"id":"S010","first_name":"Grace","last_name":"Lee","grade":"11"},"parent":{},"tracker_values":{` + trackerJSON + `}}`
	req := reqWithSession(http.MethodPost, "/api/v1/user/profile", body, app, "user", "student", "S010")
	w := doReq(app.HandleUserProfile, req)
	resp := mustDecode(t, w)
	if resp["ok"] != true {
		t.Fatalf("save failed: %v", resp)
	}

	// Load and verify tracker values come back
	req = reqWithSession(http.MethodGet, "/api/v1/user/profile?student_id=S010", "", app, "user", "student", "S010")
	w = doReq(app.HandleUserProfile, req)
	resp = mustDecode(t, w)

	if resp["ok"] != true {
		t.Fatalf("load failed: %v", resp)
	}

	values, ok := resp["tracker_values"].(map[string]any)
	if !ok {
		t.Fatal("tracker_values should be a map")
	}

	gpaVal, _ := values[jsonInt(gpaID)].(string)
	if gpaVal != "3.95" {
		t.Errorf("expected GPA value '3.95', got %q", gpaVal)
	}

	satVal, _ := values[jsonInt(satID)].(string)
	if satVal != "E:750 M:780 T:1530" {
		t.Errorf("expected SAT value, got %q", satVal)
	}
}

func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
