package datastore

import (
	"testing"
)

func TestHeaderKey(t *testing.T) {
	cases := map[string]string{
		"Student Name":    "student name",
		"  Graduate ":     "graduate",
		"Parent\tPhone":   "parent phone",
		"ENGLISH   NAME":  "english name",
		"":                "",
		"Home  Address  ": "home address",
	}
	for in, want := range cases {
		if got := headerKey(in); got != want {
			t.Errorf("headerKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConvertHeaderNamelistRows_NewLayout(t *testing.T) {
	// Mirrors the new raw/Student namelist_5_15_2026.xlsx layout: 13 columns,
	// row-index leading column, Parent Phone *before* Parent Email, no Student
	// Phone, no Home Address. "Graduate " has a trailing space.
	rows := [][]string{
		{"", "Student Name", "English Name", "School", "Grade", "Package", "Student Email", "Parent Name", "Parent Phone", "Parent Email", "Major", "Enroll", "Graduate "},
		{"1", "Lily Hou*", "Lily", "Canada", "11", "College Planning", "houlilly88@gmail.com", "Hui Li", "-", "Annahou@hotmail.com", "Bio, Medical", "2022 Fall", "2027"},
		{"2", "Ziyin Liu", "Penny", "Valley Christian", "11", "College Planning", "liuziyinnnnnnn@gmail.com", "Qiuhong Du", "925-819-4056", "21964632@qq.com", "Business or Medical", "2023 Fall", "2027"},
		{"", "", "", "", "", "", "", "", "", "", "", "", ""},
	}

	out, err := ConvertHeaderNamelistRows(rows)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 rows (header + 2 data), got %d", len(out))
	}
	if len(out[0]) != 16 {
		t.Fatalf("expected 16 output columns, got %d", len(out[0]))
	}

	// Header row mirrors canonicalNamelistHeaders.
	if out[0][2] != "Student Name" || out[0][15] != "Graduate" {
		t.Errorf("header row mismatch: %v", out[0])
	}

	// Round-trip through ParseNamelist to confirm fields land in the right slots.
	entries := ParseNamelist(out)
	if len(entries) != 2 {
		t.Fatalf("ParseNamelist saw %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.StudentName != "Lily Hou*" || e.EnglishName != "Lily" {
		t.Errorf("row 0 name/english: %q / %q", e.StudentName, e.EnglishName)
	}
	if e.School != "Canada" || e.Grade != "11" || e.Package != "College Planning" {
		t.Errorf("row 0 school/grade/package: %q %q %q", e.School, e.Grade, e.Package)
	}
	if e.Email != "houlilly88@gmail.com" {
		t.Errorf("row 0 Email = %q", e.Email)
	}
	if e.ParentName != "Hui Li" || e.ParentEmail != "Annahou@hotmail.com" || e.ParentPhone != "-" {
		t.Errorf("row 0 parent fields: name=%q email=%q phone=%q", e.ParentName, e.ParentEmail, e.ParentPhone)
	}
	if e.Major != "Bio, Medical" || e.EnrollTerm != "2022 Fall" || e.Graduation != "2027" {
		t.Errorf("row 0 major/enroll/grad: %q %q %q", e.Major, e.EnrollTerm, e.Graduation)
	}
	if e.Phone != "" || e.Address != "" {
		t.Errorf("row 0 expected empty Phone/Address, got %q / %q", e.Phone, e.Address)
	}

	e2 := entries[1]
	if e2.ParentPhone != "925-819-4056" || e2.ParentEmail != "21964632@qq.com" {
		t.Errorf("row 1 parent contact: phone=%q email=%q", e2.ParentPhone, e2.ParentEmail)
	}
}

func TestConvertHeaderNamelistRows_LegacyLayout(t *testing.T) {
	// Headers matching the original .xls layout (Student Phone present,
	// Parent Email before Parent Phone, Home Address present). Converter must
	// remap them into the same 16-column positional shape.
	rows := [][]string{
		{"Group", "Index", "Student Name", "English Name", "School", "Grade", "Package", "Student Email", "Student Phone", "Parent Name", "Parent Email", "Parent Phone", "Home Address", "Major", "Enroll", "Graduate"},
		{"2025毕业生", "1", "Ryan Wong", "Ryan", "Quarry Lane", "12", "College App", "ryan@test.com", "555-1234", "Wei Wong", "wei@test.com", "555-5678", "123 Main St", "Engineering", "2023 Fall", "2025"},
	}

	out, err := ConvertHeaderNamelistRows(rows)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	entries := ParseNamelist(out)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Phone != "555-1234" || e.Address != "123 Main St" {
		t.Errorf("legacy row dropped Phone/Address: phone=%q address=%q", e.Phone, e.Address)
	}
	if e.ParentEmail != "wei@test.com" || e.ParentPhone != "555-5678" {
		t.Errorf("legacy parent fields mismatched: email=%q phone=%q", e.ParentEmail, e.ParentPhone)
	}
}

func TestConvertHeaderNamelistRows_MissingRequiredColumns(t *testing.T) {
	rows := [][]string{
		{"School", "Grade", "Major"},
		{"Foo High", "11", "Bio"},
	}
	if _, err := ConvertHeaderNamelistRows(rows); err == nil {
		t.Fatal("expected error when both Student Name and English Name are missing")
	}
}

func TestConvertHeaderNamelistRows_SkipsBlankRows(t *testing.T) {
	rows := [][]string{
		{"Student Name", "English Name"},
		{"", ""},
		{"Jason Zeng", "Jason"},
		{"", ""},
		{"Sirui Zhao", "Larry"},
	}
	out, err := ConvertHeaderNamelistRows(rows)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// 1 header + 2 non-blank data rows
	if len(out) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(out))
	}
	if out[1][1] != "1" || out[2][1] != "2" {
		t.Errorf("sequence numbers should reset over skipped rows: got %q / %q", out[1][1], out[2][1])
	}
}
