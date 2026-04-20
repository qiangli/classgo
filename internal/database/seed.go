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
		name, priority, recurrence, category, notes string
	}{
		// GPA tracking
		{"Weighted GPA Update", "high", "monthly", "GPA", "Record current weighted GPA"},
		{"Unweighted GPA Update", "high", "monthly", "GPA", "Record current unweighted GPA"},

		// PSAT
		{"PSAT 8/9 Score", "medium", "none", "PSAT", "Record English, Math, Total scores"},
		{"PSAT 10 Score", "medium", "none", "PSAT", "Record English, Math, Total scores"},
		{"PSAT 11 (NMSQT) Score", "high", "none", "PSAT", "Record English, Math, Total scores"},

		// SAT
		{"SAT Score (1st Attempt)", "high", "none", "SAT", "Record Month/Year, English, Math, Total"},
		{"SAT Score (2nd Attempt)", "medium", "none", "SAT", "Record Month/Year, English, Math, Total"},

		// AP
		{"AP Exam Score", "high", "none", "AP", "Record subject and score (1-5)"},
		{"AP Course (In-Progress)", "medium", "none", "AP", "Record current AP courses"},

		// Math Competition
		{"AMC/AIME Score", "medium", "none", "Math Competition", "Record grade level and score"},

		// Extracurricular
		{"Talents & Instruments", "low", "monthly", "Extracurricular", "Update instruments, art, etc."},
		{"Club Activities", "low", "monthly", "Extracurricular", "E.g., National Honor Society"},
		{"Sports", "low", "monthly", "Extracurricular", "E.g., Tennis Captain, JV"},
		{"Leadership Roles", "low", "monthly", "Extracurricular", "E.g., ASB, Student Council"},
		{"Volunteer Work", "low", "monthly", "Extracurricular", "E.g., Tutoring, Library, Retirement Home"},

		// College Prep
		{"Awards & Honors", "medium", "monthly", "College Prep", "Academic or non-academic awards"},
		{"Internship Update", "medium", "none", "College Prep", "Record internship details"},
		{"Summer Experience", "medium", "none", "College Prep", "Camps, courses, volunteering, travel"},
		{"College Major Interest", "low", "none", "College Prep", "Potential majors and career plans"},

		// Personal
		{"Hobbies & Interests", "low", "none", "Personal", "Update hobbies and interests"},
		{"Favorite Subjects", "low", "none", "Personal", "Current favorite subjects"},
	}

	for _, it := range globalItems {
		db.Exec(
			`INSERT INTO tracker_items (name, notes, priority, recurrence, category, created_by, active)
			 VALUES (?, ?, ?, ?, ?, 'admin', 1)`,
			it.name, it.notes, it.priority, it.recurrence, it.category,
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
