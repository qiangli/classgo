package database

import (
	"database/sql"
	"log"
)

// SeedSampleData inserts sample tracker items and task items for demonstration.
// It is idempotent — only inserts if the tables are empty.
func SeedSampleData(db *sql.DB) {
	// Only seed if no tracker items exist yet
	var count int
	db.QueryRow("SELECT COUNT(*) FROM tracker_items WHERE deleted = 0").Scan(&count)
	if count > 0 {
		return
	}

	globalItems := []struct {
		name, priority, recurrence, category, startDate, dueDate string
	}{
		{"Daily Warmup Exercises", "medium", "daily", "General", "2026-01-06", ""},
		{"SAT Vocabulary Quiz", "high", "weekly", "SAT Prep", "2026-01-06", ""},
		{"Reading Comprehension Log", "medium", "daily", "Reading", "2026-01-06", ""},
		{"Math Problem Set", "high", "daily", "Math", "2026-01-06", ""},
		{"Monthly Progress Review", "low", "monthly", "General", "2026-01-06", ""},
	}

	for _, it := range globalItems {
		db.Exec(
			`INSERT INTO tracker_items (name, priority, recurrence, category, start_date, due_date, created_by, active)
			 VALUES (?, ?, ?, ?, ?, NULLIF(?,''), 'admin', 1)`,
			it.name, it.priority, it.recurrence, it.category, it.startDate, it.dueDate,
		)
	}

	// Sample student-specific items (assigned by teachers)
	studentItems := []struct {
		studentID, name, priority, recurrence, category, createdBy, ownerType string
		requiresSignoff                                                       bool
	}{
		{"S001", "Algebra Chapter 5 Homework", "high", "none", "Math", "T01", "teacher", true},
		{"S001", "Essay Draft: My Summer", "medium", "none", "English", "T02", "teacher", true},
		{"S002", "Geometry Worksheet", "medium", "weekly", "Math", "T01", "teacher", true},
		{"S003", "Book Report: Charlotte's Web", "high", "none", "Reading", "T02", "teacher", true},
		{"S004", "Spelling Practice List 12", "low", "daily", "English", "T02", "teacher", true},
		{"S005", "Science Fair Proposal", "high", "none", "Science", "T01", "teacher", true},
		{"S007", "Python Coding Challenge", "medium", "weekly", "Computer Science", "T03", "teacher", true},
	}

	for _, it := range studentItems {
		db.Exec(
			`INSERT INTO student_tracker_items (student_id, name, priority, recurrence, category, created_by, owner_type, requires_signoff, active)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			it.studentID, it.name, it.priority, it.recurrence, it.category, it.createdBy, it.ownerType, it.requiresSignoff,
		)
	}

	// Sample library items (unassigned templates)
	libraryItems := []struct {
		name, priority, recurrence, category, createdBy, ownerType string
	}{
		{"Weekly Vocabulary Review", "medium", "weekly", "English", "T02", "teacher"},
		{"Math Drill Template", "high", "daily", "Math", "T01", "teacher"},
		{"Lab Report Template", "medium", "none", "Science", "T01", "teacher"},
	}

	for _, it := range libraryItems {
		db.Exec(
			`INSERT INTO student_tracker_items (student_id, name, priority, recurrence, category, created_by, owner_type, requires_signoff, active)
			 VALUES ('', ?, ?, ?, ?, ?, ?, 1, 1)`,
			it.name, it.priority, it.recurrence, it.category, it.createdBy, it.ownerType,
		)
	}

	log.Printf("Seeded sample data: %d global items, %d student items, %d library templates",
		len(globalItems), len(studentItems), len(libraryItems))
}
