package datastore

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"classgo/internal/models"
)

// ParseNamelist parses rows from a namelist .xls file into NamelistEntry structs.
// Expected column layout (0-indexed):
//
//	0: Group label (sparse)
//	1: Index/sequence (ignored)
//	2: Student Name (original/Chinese name)
//	3: English Name
//	4: School
//	5: Grade
//	6: Package
//	7: Student Email
//	8: Student Phone
//	9: Parent Name
//	10: Parent Email
//	11: Parent Phone
//	12: Home Address
//	13: Major
//	14: Enroll
//	15: Graduate
func ParseNamelist(rows [][]string) []models.NamelistEntry {
	if len(rows) < 2 {
		return nil
	}

	var entries []models.NamelistEntry
	currentGroup := ""

	// Skip header row (row 0)
	for i, row := range rows[1:] {
		// Carry forward group label
		if col(row, 0) != "" {
			currentGroup = col(row, 0)
		}

		studentName := col(row, 2)
		englishName := col(row, 3)

		// Skip empty rows (must have at least a student name or english name)
		if studentName == "" && englishName == "" {
			continue
		}

		// Parse first/last name from Student Name column
		// Rule: last word = last name, preceding words = given name
		firstName, lastName := splitName(studentName)

		entries = append(entries, models.NamelistEntry{
			RowIndex:    i + 1, // 1-based, excluding header
			GroupLabel:  currentGroup,
			StudentName: studentName,
			EnglishName: englishName,
			FirstName:   firstName,
			LastName:    lastName,
			School:      col(row, 4),
			Grade:       col(row, 5),
			Package:     col(row, 6),
			Email:       cleanEmail(col(row, 7)),
			Phone:       col(row, 8),
			ParentName:  col(row, 9),
			ParentEmail: cleanEmail(col(row, 10)),
			ParentPhone: col(row, 11),
			Address:     col(row, 12),
			Major:       col(row, 13),
			EnrollTerm:  col(row, 14),
			Graduation:  col(row, 15),
		})
	}
	return entries
}

// col safely accesses a column in a row, returning "" if out of bounds.
func col(row []string, idx int) string {
	if idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

// cleanEmail extracts a clean email address from a cell that may contain
// embedded hyperlink junk (e.g. "user@example.com(mailto:user@example.com...)").
func cleanEmail(s string) string {
	// Strip anything after a "(" which typically contains mailto: junk from .xls hyperlinks
	if idx := strings.Index(s, "("); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

// splitName splits a full name into first (given) and last (family) names.
// Rule: last word = last name, preceding words = given name.
func splitName(fullName string) (firstName, lastName string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

// PreviewNamelistImport analyzes namelist entries against existing students
// and classifies each as new, matching, or conflicting.
func PreviewNamelistImport(db *sql.DB, entries []models.NamelistEntry) (*models.NamelistPreview, error) {
	existing, err := queryStudents(db, false)
	if err != nil {
		return nil, fmt.Errorf("query students: %w", err)
	}

	// Build lookup maps
	byName := make(map[string]models.Student)  // "firstname lastname" -> student
	byEmail := make(map[string]models.Student)  // email -> student
	for _, s := range existing {
		key := strings.ToLower(s.FirstName + " " + s.LastName)
		byName[key] = s
		if s.Email != "" {
			byEmail[strings.ToLower(s.Email)] = s
		}
	}

	preview := &models.NamelistPreview{
		TotalRows: len(entries),
	}

	for _, entry := range entries {
		nameKey := strings.ToLower(entry.FirstName + " " + entry.LastName)

		// Try to find a match by name first, then by email
		var found bool
		var match models.Student

		if s, ok := byName[nameKey]; ok && nameKey != " " {
			found = true
			match = s
		} else if entry.Email != "" {
			if s, ok := byEmail[strings.ToLower(entry.Email)]; ok {
				found = true
				match = s
			}
		}

		if !found {
			preview.NewEntries = append(preview.NewEntries, entry)
			continue
		}

		// Found a match — check for differences
		diffs := computeDifferences(entry, match)
		if len(diffs) == 0 {
			preview.Matches = append(preview.Matches, entry)
		} else {
			preview.Conflicts = append(preview.Conflicts, models.NamelistConflict{
				Entry:       entry,
				ExistingID:  match.ID,
				Existing:    match,
				Differences: diffs,
			})
		}
	}

	return preview, nil
}

// computeDifferences returns a list of field names that differ between a namelist entry
// and an existing student record.
func computeDifferences(entry models.NamelistEntry, existing models.Student) []string {
	var diffs []string
	check := func(field, newVal, oldVal string) {
		if newVal != "" && !strings.EqualFold(newVal, oldVal) {
			diffs = append(diffs, field)
		}
	}
	check("email", entry.Email, existing.Email)
	check("phone", entry.Phone, existing.Phone)
	check("school", entry.School, existing.School)
	check("grade", entry.Grade, existing.Grade)
	check("address", entry.Address, existing.Address)
	check("english_name", entry.EnglishName, existing.EnglishName)
	check("package", entry.Package, existing.Package)
	check("major", entry.Major, existing.Major)
	check("enroll_term", entry.EnrollTerm, existing.EnrollTerm)
	check("graduation", entry.Graduation, existing.Graduation)
	return diffs
}

// GenerateStudentID generates the next student ID with pattern YYnnn.
// year is the full year (e.g. 2026). Returns e.g. "26001".
func GenerateStudentID(db *sql.DB, year int) (string, error) {
	prefix := fmt.Sprintf("%02d", year%100)
	pattern := prefix + "[0-9][0-9][0-9]"

	var maxID sql.NullString
	err := db.QueryRow(
		"SELECT MAX(id) FROM students WHERE id GLOB ?", pattern,
	).Scan(&maxID)
	if err != nil {
		return "", err
	}

	seq := 1
	if maxID.Valid && len(maxID.String) == 5 {
		fmt.Sscanf(maxID.String[2:], "%d", &seq)
		seq++
	}

	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// GenerateParentID generates the next parent ID with pattern Pnnn.
func GenerateParentID(db *sql.DB) (string, error) {
	var maxID sql.NullString
	err := db.QueryRow(
		"SELECT MAX(id) FROM parents WHERE id GLOB 'P[0-9][0-9][0-9]'",
	).Scan(&maxID)
	if err != nil {
		return "", err
	}

	seq := 1
	if maxID.Valid && len(maxID.String) == 4 {
		fmt.Sscanf(maxID.String[1:], "%d", &seq)
		seq++
	}

	return fmt.Sprintf("P%03d", seq), nil
}

// ExecuteNamelistImport imports namelist entries into the database.
// decisions maps row_index to action: "insert", "merge", or "skip".
func ExecuteNamelistImport(db *sql.DB, entries []models.NamelistEntry, decisions map[int]string) (*models.NamelistImportResult, error) {
	// Re-run preview to get current state
	preview, err := PreviewNamelistImport(db, entries)
	if err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result := &models.NamelistImportResult{}
	year := time.Now().Year()

	// Build conflict lookup by row index
	conflictMap := make(map[int]models.NamelistConflict)
	for _, c := range preview.Conflicts {
		conflictMap[c.Entry.RowIndex] = c
	}

	// Process new entries (always insert)
	for _, entry := range preview.NewEntries {
		action := decisions[entry.RowIndex]
		if action == "skip" {
			result.StudentsSkipped++
			continue
		}

		id, err := generateStudentIDInTx(tx, year)
		if err != nil {
			return nil, fmt.Errorf("generate student ID: %w", err)
		}

		parentID, err := upsertParentInTx(tx, entry, result)
		if err != nil {
			return nil, fmt.Errorf("upsert parent: %w", err)
		}

		if err := insertStudentInTx(tx, id, parentID, entry); err != nil {
			return nil, fmt.Errorf("insert student %s: %w", id, err)
		}
		result.StudentsInserted++
	}

	// Process conflicts based on decisions
	for _, conflict := range preview.Conflicts {
		action := decisions[conflict.Entry.RowIndex]
		switch action {
		case "merge":
			parentID, err := upsertParentInTx(tx, conflict.Entry, result)
			if err != nil {
				return nil, fmt.Errorf("upsert parent: %w", err)
			}
			if err := mergeStudentInTx(tx, conflict.ExistingID, parentID, conflict.Entry); err != nil {
				return nil, fmt.Errorf("merge student %s: %w", conflict.ExistingID, err)
			}
			result.StudentsMerged++
		case "insert":
			id, err := generateStudentIDInTx(tx, year)
			if err != nil {
				return nil, fmt.Errorf("generate student ID: %w", err)
			}
			parentID, err := upsertParentInTx(tx, conflict.Entry, result)
			if err != nil {
				return nil, fmt.Errorf("upsert parent: %w", err)
			}
			if err := insertStudentInTx(tx, id, parentID, conflict.Entry); err != nil {
				return nil, fmt.Errorf("insert student %s: %w", id, err)
			}
			result.StudentsInserted++
		default: // "skip" or unspecified
			result.StudentsSkipped++
		}
	}

	// Process exact matches — still merge parents
	for _, entry := range preview.Matches {
		if entry.ParentName != "" || entry.ParentEmail != "" {
			// Find the matched student to get their ID for parent linking
			existing, err := queryStudents(db, false)
			if err == nil {
				nameKey := strings.ToLower(entry.FirstName + " " + entry.LastName)
				for _, s := range existing {
					key := strings.ToLower(s.FirstName + " " + s.LastName)
					if key == nameKey {
						parentID, pErr := upsertParentInTx(tx, entry, result)
						if pErr == nil && parentID != "" && s.ParentID != parentID {
							tx.Exec("UPDATE students SET parent_id = ? WHERE id = ?", parentID, s.ID)
						}
						break
					}
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}

func generateStudentIDInTx(tx *sql.Tx, year int) (string, error) {
	prefix := fmt.Sprintf("%02d", year%100)
	pattern := prefix + "[0-9][0-9][0-9]"

	var maxID sql.NullString
	err := tx.QueryRow("SELECT MAX(id) FROM students WHERE id GLOB ?", pattern).Scan(&maxID)
	if err != nil {
		return "", err
	}

	seq := 1
	if maxID.Valid && len(maxID.String) == 5 {
		fmt.Sscanf(maxID.String[2:], "%d", &seq)
		seq++
	}

	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

func insertStudentInTx(tx *sql.Tx, id, parentID string, entry models.NamelistEntry) error {
	_, err := tx.Exec(
		`INSERT INTO students (id, first_name, last_name, grade, school, parent_id, email, phone, address,
		 english_name, package, major, enroll_term, graduation, active, deleted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 0)`,
		id, entry.FirstName, entry.LastName, entry.Grade, entry.School, parentID,
		entry.Email, entry.Phone, entry.Address,
		entry.EnglishName, entry.Package, entry.Major, entry.EnrollTerm, entry.Graduation,
	)
	return err
}

func mergeStudentInTx(tx *sql.Tx, existingID, parentID string, entry models.NamelistEntry) error {
	setClauses := []string{}
	args := []any{}

	set := func(col, val string) {
		if val != "" {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
	}

	set("first_name", entry.FirstName)
	set("last_name", entry.LastName)
	set("grade", entry.Grade)
	set("school", entry.School)
	set("email", entry.Email)
	set("phone", entry.Phone)
	set("address", entry.Address)
	set("english_name", entry.EnglishName)
	set("package", entry.Package)
	set("major", entry.Major)
	set("enroll_term", entry.EnrollTerm)
	set("graduation", entry.Graduation)

	if parentID != "" {
		setClauses = append(setClauses, "parent_id = ?")
		args = append(args, parentID)
	}

	if len(setClauses) == 0 {
		return nil
	}

	args = append(args, existingID)
	_, err := tx.Exec(
		"UPDATE students SET "+strings.Join(setClauses, ", ")+" WHERE id = ?",
		args...,
	)
	return err
}

// upsertParentInTx finds or creates a parent record from namelist entry data.
// Always merges: if a parent with matching name+email exists, update; otherwise create.
func upsertParentInTx(tx *sql.Tx, entry models.NamelistEntry, result *models.NamelistImportResult) (string, error) {
	if entry.ParentName == "" && entry.ParentEmail == "" {
		return "", nil
	}

	parentFirst, parentLast := splitName(entry.ParentName)

	// Try to find existing parent by name+email
	var existingID string
	if entry.ParentEmail != "" && parentFirst != "" {
		err := tx.QueryRow(
			"SELECT id FROM parents WHERE LOWER(first_name) = LOWER(?) AND LOWER(last_name) = LOWER(?) AND LOWER(email) = LOWER(?)",
			parentFirst, parentLast, entry.ParentEmail,
		).Scan(&existingID)
		if err == nil {
			// Found — update with any new data
			if entry.ParentPhone != "" {
				tx.Exec("UPDATE parents SET phone = ? WHERE id = ? AND (phone IS NULL OR phone = '')", entry.ParentPhone, existingID)
			}
			result.ParentsUpdated++
			return existingID, nil
		}
	}

	// Also try matching by email alone
	if entry.ParentEmail != "" {
		err := tx.QueryRow(
			"SELECT id FROM parents WHERE LOWER(email) = LOWER(?)",
			entry.ParentEmail,
		).Scan(&existingID)
		if err == nil {
			// Update name/phone if needed
			if parentFirst != "" {
				tx.Exec("UPDATE parents SET first_name = ?, last_name = ? WHERE id = ?", parentFirst, parentLast, existingID)
			}
			if entry.ParentPhone != "" {
				tx.Exec("UPDATE parents SET phone = ? WHERE id = ? AND (phone IS NULL OR phone = '')", entry.ParentPhone, existingID)
			}
			result.ParentsUpdated++
			return existingID, nil
		}
	}

	// Not found — create new parent
	newID, err := generateParentIDInTx(tx)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(
		"INSERT INTO parents (id, first_name, last_name, email, phone, deleted) VALUES (?, ?, ?, ?, ?, 0)",
		newID, parentFirst, parentLast, entry.ParentEmail, entry.ParentPhone,
	)
	if err != nil {
		return "", err
	}
	result.ParentsCreated++
	return newID, nil
}

func generateParentIDInTx(tx *sql.Tx) (string, error) {
	var maxID sql.NullString
	err := tx.QueryRow("SELECT MAX(id) FROM parents WHERE id GLOB 'P[0-9][0-9][0-9]'").Scan(&maxID)
	if err != nil {
		return "", err
	}

	seq := 1
	if maxID.Valid && len(maxID.String) == 4 {
		fmt.Sscanf(maxID.String[1:], "%d", &seq)
		seq++
	}

	return fmt.Sprintf("P%03d", seq), nil
}
