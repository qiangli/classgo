package datastore

import (
	"database/sql"
	"os"
	"testing"

	"classgo/internal/database"
	"classgo/internal/models"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "classgo-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	db, err := database.OpenDB(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.MigrateDB(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSplitName(t *testing.T) {
	tests := []struct {
		input     string
		wantFirst string
		wantLast  string
	}{
		{"Laurisa Chou", "Laurisa", "Chou"},
		{"Ryan Wong", "Ryan", "Wong"},
		{"Huiwen Huang", "Huiwen", "Huang"},
		{"Mary Jane Watson", "Mary Jane", "Watson"},
		{"Madonna", "Madonna", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		first, last := splitName(tt.input)
		if first != tt.wantFirst || last != tt.wantLast {
			t.Errorf("splitName(%q) = (%q, %q), want (%q, %q)", tt.input, first, last, tt.wantFirst, tt.wantLast)
		}
	}
}

func TestCleanEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "user@example.com"},
		{"user@example.com(mailto:user@example.com\x00junk)", "user@example.com"},
		{"", ""},
		{"no-parens@test.com", "no-parens@test.com"},
	}
	for _, tt := range tests {
		got := cleanEmail(tt.input)
		if got != tt.want {
			t.Errorf("cleanEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseNamelist(t *testing.T) {
	rows := [][]string{
		{"", "", "Student Name", "English Name", "School", "Grade", "Package", "Student Email", "Student Phone", "Parent Name", "Parent Email", "Parent Phone", "Home Address", "Major", "Enroll", "Graduate"},
		{"2025毕业生", "1", "Laurisa Chou", "Laurisa", "Dublin High", "12", "College App", "laurisa@test.com", "", "", "parent@test.com", "", "", "CS", "2023 Fall", "2025"},
		{"", "2", "Ryan Wong", "Ryan", "Quarry Lane", "12", "College App", "ryan@test.com", "555-1234", "Wei Wong", "wei@test.com", "555-5678", "123 Main St", "Engineering", "2023 Fall", "2025"},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"2026届", "3", "Alice Zhang", "Alice", "Amador", "11", "SAT Prep", "alice@test.com", "", "", "", "", "", "Biology", "2024 Fall", "2026"},
	}

	entries := ParseNamelist(rows)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Entry 1
	e := entries[0]
	if e.StudentName != "Laurisa Chou" {
		t.Errorf("entry 0 StudentName = %q", e.StudentName)
	}
	if e.FirstName != "Laurisa" || e.LastName != "Chou" {
		t.Errorf("entry 0 name split: first=%q last=%q", e.FirstName, e.LastName)
	}
	if e.GroupLabel != "2025毕业生" {
		t.Errorf("entry 0 GroupLabel = %q", e.GroupLabel)
	}
	if e.Email != "laurisa@test.com" {
		t.Errorf("entry 0 Email = %q", e.Email)
	}
	if e.ParentEmail != "parent@test.com" {
		t.Errorf("entry 0 ParentEmail = %q", e.ParentEmail)
	}
	if e.Major != "CS" {
		t.Errorf("entry 0 Major = %q", e.Major)
	}

	// Entry 2: group label carried forward
	if entries[1].GroupLabel != "2025毕业生" {
		t.Errorf("entry 1 GroupLabel not carried forward: %q", entries[1].GroupLabel)
	}
	if entries[1].ParentName != "Wei Wong" {
		t.Errorf("entry 1 ParentName = %q", entries[1].ParentName)
	}

	// Entry 3: new group label
	if entries[2].GroupLabel != "2026届" {
		t.Errorf("entry 2 GroupLabel = %q", entries[2].GroupLabel)
	}
}

func TestParseNamelist_EmptyRows(t *testing.T) {
	rows := [][]string{
		{"", "", "Student Name", "English Name"},
	}
	entries := ParseNamelist(rows)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for header-only, got %d", len(entries))
	}
}

func TestGenerateStudentID(t *testing.T) {
	db := setupTestDB(t)

	// No existing students — should get 26001
	id, err := GenerateStudentID(db, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if id != "26001" {
		t.Errorf("expected 26001, got %q", id)
	}

	// Insert a student with that ID
	db.Exec("INSERT INTO students (id, first_name, last_name) VALUES (?, ?, ?)", "26001", "Test", "One")

	// Next should be 26002
	id, err = GenerateStudentID(db, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if id != "26002" {
		t.Errorf("expected 26002, got %q", id)
	}

	// Different year
	id, err = GenerateStudentID(db, 2027)
	if err != nil {
		t.Fatal(err)
	}
	if id != "27001" {
		t.Errorf("expected 27001, got %q", id)
	}
}

func TestGenerateParentID(t *testing.T) {
	db := setupTestDB(t)

	id, err := GenerateParentID(db)
	if err != nil {
		t.Fatal(err)
	}
	if id != "P001" {
		t.Errorf("expected P001, got %q", id)
	}

	db.Exec("INSERT INTO parents (id, first_name, last_name) VALUES (?, ?, ?)", "P001", "Test", "Parent")

	id, err = GenerateParentID(db)
	if err != nil {
		t.Fatal(err)
	}
	if id != "P002" {
		t.Errorf("expected P002, got %q", id)
	}
}

func TestPreviewNamelistImport_NewEntries(t *testing.T) {
	db := setupTestDB(t)

	entries := []models.NamelistEntry{
		{RowIndex: 1, StudentName: "New Student", FirstName: "New", LastName: "Student", Email: "new@test.com"},
		{RowIndex: 2, StudentName: "Another One", FirstName: "Another", LastName: "One", Email: "another@test.com"},
	}

	preview, err := PreviewNamelistImport(db, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.NewEntries) != 2 {
		t.Errorf("expected 2 new entries, got %d", len(preview.NewEntries))
	}
	if len(preview.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(preview.Conflicts))
	}
	if len(preview.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(preview.Matches))
	}
}

func TestPreviewNamelistImport_ExactMatch(t *testing.T) {
	db := setupTestDB(t)

	// Insert existing student
	db.Exec("INSERT INTO students (id, first_name, last_name, email, school, grade, active) VALUES (?, ?, ?, ?, ?, ?, 1)",
		"S001", "Alice", "Wang", "alice@test.com", "Dublin High", "10")

	entries := []models.NamelistEntry{
		{RowIndex: 1, FirstName: "Alice", LastName: "Wang", Email: "alice@test.com", School: "Dublin High", Grade: "10"},
	}

	preview, err := PreviewNamelistImport(db, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(preview.Matches))
	}
	if len(preview.NewEntries) != 0 {
		t.Errorf("expected 0 new entries, got %d", len(preview.NewEntries))
	}
}

func TestPreviewNamelistImport_Conflict(t *testing.T) {
	db := setupTestDB(t)

	// Insert existing student with different email
	db.Exec("INSERT INTO students (id, first_name, last_name, email, school, active) VALUES (?, ?, ?, ?, ?, 1)",
		"S001", "Alice", "Wang", "old@test.com", "Dublin High")

	entries := []models.NamelistEntry{
		{RowIndex: 1, FirstName: "Alice", LastName: "Wang", Email: "new@test.com", School: "Dublin High"},
	}

	preview, err := PreviewNamelistImport(db, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(preview.Conflicts))
	}
	if len(preview.Conflicts) > 0 {
		c := preview.Conflicts[0]
		if c.ExistingID != "S001" {
			t.Errorf("conflict ExistingID = %q", c.ExistingID)
		}
		found := false
		for _, d := range c.Differences {
			if d == "email" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'email' in differences, got %v", c.Differences)
		}
	}
}

func TestExecuteNamelistImport_Insert(t *testing.T) {
	db := setupTestDB(t)

	entries := []models.NamelistEntry{
		{
			RowIndex: 1, StudentName: "Test Student", FirstName: "Test", LastName: "Student",
			Email: "test@example.com", School: "Test High", Grade: "10",
			EnglishName: "Tester", Package: "SAT Prep", Major: "CS",
			EnrollTerm: "2024 Fall", Graduation: "2026",
			ParentName: "Wei Student", ParentEmail: "wei@example.com", ParentPhone: "555-1234",
		},
	}

	decisions := map[int]string{1: "insert"}

	result, err := ExecuteNamelistImport(db, entries, decisions)
	if err != nil {
		t.Fatal(err)
	}
	if result.StudentsInserted != 1 {
		t.Errorf("expected 1 inserted, got %d", result.StudentsInserted)
	}
	if result.ParentsCreated != 1 {
		t.Errorf("expected 1 parent created, got %d", result.ParentsCreated)
	}

	// Verify student was created with correct fields
	var firstName, lastName, email, englishName, pkg, major, enrollTerm, graduation, parentID string
	err = db.QueryRow("SELECT first_name, last_name, email, COALESCE(english_name,''), COALESCE(package,''), COALESCE(major,''), COALESCE(enroll_term,''), COALESCE(graduation,''), COALESCE(parent_id,'') FROM students WHERE first_name = 'Test' AND last_name = 'Student'").
		Scan(&firstName, &lastName, &email, &englishName, &pkg, &major, &enrollTerm, &graduation, &parentID)
	if err != nil {
		t.Fatalf("student not found: %v", err)
	}
	if email != "test@example.com" {
		t.Errorf("email = %q", email)
	}
	if englishName != "Tester" {
		t.Errorf("english_name = %q", englishName)
	}
	if pkg != "SAT Prep" {
		t.Errorf("package = %q", pkg)
	}
	if major != "CS" {
		t.Errorf("major = %q", major)
	}
	if enrollTerm != "2024 Fall" {
		t.Errorf("enroll_term = %q", enrollTerm)
	}
	if graduation != "2026" {
		t.Errorf("graduation = %q", graduation)
	}

	// Verify parent was created and linked
	if parentID == "" {
		t.Error("parent_id not set on student")
	}
	var parentFirst, parentLast, parentEmail string
	err = db.QueryRow("SELECT first_name, last_name, email FROM parents WHERE id = ?", parentID).
		Scan(&parentFirst, &parentLast, &parentEmail)
	if err != nil {
		t.Fatalf("parent not found: %v", err)
	}
	if parentFirst != "Wei" || parentLast != "Student" {
		t.Errorf("parent name = %q %q", parentFirst, parentLast)
	}
	if parentEmail != "wei@example.com" {
		t.Errorf("parent email = %q", parentEmail)
	}
}

func TestExecuteNamelistImport_Merge(t *testing.T) {
	db := setupTestDB(t)

	// Insert existing student
	db.Exec("INSERT INTO students (id, first_name, last_name, email, school, grade, active) VALUES (?, ?, ?, ?, ?, ?, 1)",
		"S001", "Alice", "Wang", "old@test.com", "Old School", "9")

	entries := []models.NamelistEntry{
		{
			RowIndex: 1, FirstName: "Alice", LastName: "Wang",
			Email: "new@test.com", School: "New School", Grade: "10",
			Major: "Biology",
		},
	}

	decisions := map[int]string{1: "merge"}

	result, err := ExecuteNamelistImport(db, entries, decisions)
	if err != nil {
		t.Fatal(err)
	}
	if result.StudentsMerged != 1 {
		t.Errorf("expected 1 merged, got %d", result.StudentsMerged)
	}

	// Verify fields were updated
	var email, school, grade, major string
	err = db.QueryRow("SELECT email, school, grade, COALESCE(major,'') FROM students WHERE id = 'S001'").
		Scan(&email, &school, &grade, &major)
	if err != nil {
		t.Fatal(err)
	}
	if email != "new@test.com" {
		t.Errorf("email not updated: %q", email)
	}
	if school != "New School" {
		t.Errorf("school not updated: %q", school)
	}
	if grade != "10" {
		t.Errorf("grade not updated: %q", grade)
	}
	if major != "Biology" {
		t.Errorf("major not set: %q", major)
	}
}

func TestExecuteNamelistImport_Skip(t *testing.T) {
	db := setupTestDB(t)

	entries := []models.NamelistEntry{
		{RowIndex: 1, FirstName: "Skip", LastName: "Me", Email: "skip@test.com"},
	}

	decisions := map[int]string{1: "skip"}

	result, err := ExecuteNamelistImport(db, entries, decisions)
	if err != nil {
		t.Fatal(err)
	}
	if result.StudentsSkipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.StudentsSkipped)
	}

	// Verify student was NOT created
	var count int
	db.QueryRow("SELECT COUNT(*) FROM students WHERE first_name = 'Skip'").Scan(&count)
	if count != 0 {
		t.Error("student should not have been created")
	}
}

func TestExecuteNamelistImport_ParentMerge(t *testing.T) {
	db := setupTestDB(t)

	// Insert existing parent
	db.Exec("INSERT INTO parents (id, first_name, last_name, email, deleted) VALUES (?, ?, ?, ?, 0)",
		"P001", "Wei", "Wang", "wei@test.com")

	entries := []models.NamelistEntry{
		{
			RowIndex: 1, FirstName: "Alice", LastName: "Wang",
			Email:      "alice@test.com",
			ParentName: "Wei Wang", ParentEmail: "wei@test.com", ParentPhone: "555-9999",
		},
	}

	decisions := map[int]string{1: "insert"}

	result, err := ExecuteNamelistImport(db, entries, decisions)
	if err != nil {
		t.Fatal(err)
	}
	if result.StudentsInserted != 1 {
		t.Errorf("expected 1 inserted, got %d", result.StudentsInserted)
	}
	if result.ParentsUpdated != 1 {
		t.Errorf("expected 1 parent updated, got %d", result.ParentsUpdated)
	}
	if result.ParentsCreated != 0 {
		t.Errorf("expected 0 parents created, got %d", result.ParentsCreated)
	}

	// Verify student is linked to existing parent
	var parentID string
	db.QueryRow("SELECT COALESCE(parent_id,'') FROM students WHERE first_name = 'Alice'").Scan(&parentID)
	if parentID != "P001" {
		t.Errorf("expected parent_id P001, got %q", parentID)
	}
}

func TestExecuteNamelistImport_StudentIDFormat(t *testing.T) {
	db := setupTestDB(t)

	entries := []models.NamelistEntry{
		{RowIndex: 1, FirstName: "First", LastName: "Student", Email: "first@test.com"},
		{RowIndex: 2, FirstName: "Second", LastName: "Student", Email: "second@test.com"},
	}

	decisions := map[int]string{1: "insert", 2: "insert"}

	result, err := ExecuteNamelistImport(db, entries, decisions)
	if err != nil {
		t.Fatal(err)
	}
	if result.StudentsInserted != 2 {
		t.Fatalf("expected 2 inserted, got %d", result.StudentsInserted)
	}

	// Verify IDs follow YYnnn pattern
	rows, err := db.Query("SELECT id FROM students ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 students, got %d", len(ids))
	}
	// Both should have 5-char YYnnn format
	for _, id := range ids {
		if len(id) != 5 {
			t.Errorf("expected 5-char ID, got %q", id)
		}
	}
	// IDs should be sequential
	if ids[0] >= ids[1] {
		t.Errorf("IDs not sequential: %v", ids)
	}
}

// TestReadXLSFile_ActualFile tests the .xls reader against the real namelist file if present.
func TestReadXLSFile_ActualFile(t *testing.T) {
	path := "../../raw/Student NameList_update_12_15_2025.xls"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("raw namelist file not present")
	}

	rows, err := ReadXLSFile(path)
	if err != nil {
		t.Fatalf("ReadXLSFile: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(rows))
	}

	entries := ParseNamelist(rows)
	if len(entries) == 0 {
		t.Fatal("expected entries from real file")
	}

	// Verify no emails have junk
	for _, e := range entries {
		if len(e.Email) > 0 && e.Email[len(e.Email)-1] == ')' {
			t.Errorf("email has junk: %q (row %d)", e.Email, e.RowIndex)
		}
		if len(e.ParentEmail) > 0 && e.ParentEmail[len(e.ParentEmail)-1] == ')' {
			t.Errorf("parent email has junk: %q (row %d)", e.ParentEmail, e.RowIndex)
		}
	}

	// Verify names were split
	for _, e := range entries {
		if e.StudentName != "" && e.FirstName == "" && e.LastName == "" {
			t.Errorf("name not split for %q (row %d)", e.StudentName, e.RowIndex)
		}
	}

	t.Logf("Parsed %d entries from real file", len(entries))
}

// TestFullImportFlow tests the complete flow: parse -> preview -> execute with a real-like dataset.
func TestFullImportFlow(t *testing.T) {
	db := setupTestDB(t)

	// Seed some existing students
	db.Exec("INSERT INTO students (id, first_name, last_name, email, school, grade, active) VALUES (?, ?, ?, ?, ?, ?, 1)",
		"S001", "Alice", "Wang", "alice@old.com", "Dublin High", "10")
	db.Exec("INSERT INTO students (id, first_name, last_name, email, school, grade, english_name, active) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
		"S002", "Bob", "Johnson", "bob@test.com", "Quarry Lane", "11", "Bob")

	rows := [][]string{
		{"", "", "Student Name", "English Name", "School", "Grade", "Package", "Student Email", "Student Phone", "Parent Name", "Parent Email", "Parent Phone", "Home Address", "Major", "Enroll", "Graduate"},
		// Conflict: same name, different email
		{"", "", "Alice Wang", "Alice", "New School", "11", "College App", "alice@new.com", "", "Wei Wang", "wei@test.com", "", "", "CS", "2024 Fall", "2026"},
		// Exact match
		{"", "", "Bob Johnson", "Bob", "Quarry Lane", "11", "", "bob@test.com", "", "", "", "", "", "", "", ""},
		// New student
		{"", "", "Charlie Zhang", "Charlie", "Amador", "10", "SAT Prep", "charlie@test.com", "", "Li Zhang", "li@test.com", "555-0001", "", "Bio", "2025 Fall", "2027"},
	}

	entries := ParseNamelist(rows)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Preview
	preview, err := PreviewNamelistImport(db, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(preview.Conflicts))
	}
	if len(preview.Matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(preview.Matches))
	}
	if len(preview.NewEntries) != 1 {
		t.Errorf("expected 1 new, got %d", len(preview.NewEntries))
	}

	// Execute: merge the conflict, insert the new one
	decisions := map[int]string{
		entries[0].RowIndex: "merge",  // Alice - merge
		entries[2].RowIndex: "insert", // Charlie - insert
	}

	result, err := ExecuteNamelistImport(db, entries, decisions)
	if err != nil {
		t.Fatal(err)
	}
	if result.StudentsMerged != 1 {
		t.Errorf("expected 1 merged, got %d", result.StudentsMerged)
	}
	if result.StudentsInserted != 1 {
		t.Errorf("expected 1 inserted, got %d", result.StudentsInserted)
	}
	if result.ParentsCreated < 1 {
		t.Errorf("expected at least 1 parent created, got %d", result.ParentsCreated)
	}

	// Verify Alice was updated
	var aliceEmail, aliceSchool, aliceGrade string
	db.QueryRow("SELECT email, school, grade FROM students WHERE id = 'S001'").Scan(&aliceEmail, &aliceSchool, &aliceGrade)
	if aliceEmail != "alice@new.com" {
		t.Errorf("Alice email not updated: %q", aliceEmail)
	}
	if aliceSchool != "New School" {
		t.Errorf("Alice school not updated: %q", aliceSchool)
	}

	// Verify Charlie was created
	var charlieCount int
	db.QueryRow("SELECT COUNT(*) FROM students WHERE first_name = 'Charlie' AND last_name = 'Zhang'").Scan(&charlieCount)
	if charlieCount != 1 {
		t.Errorf("Charlie not found in DB")
	}

	// Verify total student count
	var totalStudents int
	db.QueryRow("SELECT COUNT(*) FROM students").Scan(&totalStudents)
	if totalStudents != 3 {
		t.Errorf("expected 3 total students, got %d", totalStudents)
	}
}
