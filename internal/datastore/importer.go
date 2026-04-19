package datastore

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"classgo/internal/models"
)

// ImportAll imports all entity data into SQLite index tables.
func ImportAll(db *sql.DB, data *EntityData) error {
	if err := importParents(db, data.Parents); err != nil {
		return fmt.Errorf("import parents: %w", err)
	}
	if err := importStudents(db, data.Students); err != nil {
		return fmt.Errorf("import students: %w", err)
	}
	if err := importTeachers(db, data.Teachers); err != nil {
		return fmt.Errorf("import teachers: %w", err)
	}
	if err := importRooms(db, data.Rooms); err != nil {
		return fmt.Errorf("import rooms: %w", err)
	}
	if err := importSchedules(db, data.Schedules); err != nil {
		return fmt.Errorf("import schedules: %w", err)
	}
	log.Printf("Imported: %d parents, %d students, %d teachers, %d rooms, %d schedules",
		len(data.Parents), len(data.Students), len(data.Teachers), len(data.Rooms), len(data.Schedules))
	return nil
}

func rowHash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", h[:8])
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func importParents(db *sql.DB, parents []models.Parent) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Collect IDs for deletion of removed rows
	ids := make(map[string]bool)
	for _, p := range parents {
		ids[p.ID] = true
		hash := rowHash(p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Notes)
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO parents (id, first_name, last_name, email, phone, notes, row_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Notes, hash,
		)
		if err != nil {
			return err
		}
	}
	if err := deleteRemovedRows(tx, "parents", ids); err != nil {
		return err
	}
	return tx.Commit()
}

func importStudents(db *sql.DB, students []models.Student) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids := make(map[string]bool)
	for _, s := range students {
		ids[s.ID] = true
		hash := rowHash(s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Notes, fmt.Sprint(s.Active))
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO students (id, first_name, last_name, grade, school, parent_id, notes, active, row_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Notes, boolToInt(s.Active), hash,
		)
		if err != nil {
			return err
		}
	}
	if err := deleteRemovedRows(tx, "students", ids); err != nil {
		return err
	}
	return tx.Commit()
}

func importTeachers(db *sql.DB, teachers []models.Teacher) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids := make(map[string]bool)
	for _, t := range teachers {
		ids[t.ID] = true
		subjects := strings.Join(t.Subjects, ";")
		hash := rowHash(t.ID, t.FirstName, t.LastName, t.Email, t.Phone, subjects, fmt.Sprint(t.Active))
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO teachers (id, first_name, last_name, email, phone, subjects, active, row_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.FirstName, t.LastName, t.Email, t.Phone, subjects, boolToInt(t.Active), hash,
		)
		if err != nil {
			return err
		}
	}
	if err := deleteRemovedRows(tx, "teachers", ids); err != nil {
		return err
	}
	return tx.Commit()
}

func importRooms(db *sql.DB, rooms []models.Room) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids := make(map[string]bool)
	for _, r := range rooms {
		ids[r.ID] = true
		hash := rowHash(r.ID, r.Name, fmt.Sprint(r.Capacity), r.Notes)
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO rooms (id, name, capacity, notes, row_hash)
			 VALUES (?, ?, ?, ?, ?)`,
			r.ID, r.Name, r.Capacity, r.Notes, hash,
		)
		if err != nil {
			return err
		}
	}
	if err := deleteRemovedRows(tx, "rooms", ids); err != nil {
		return err
	}
	return tx.Commit()
}

func importSchedules(db *sql.DB, schedules []models.Schedule) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids := make(map[string]bool)
	for _, s := range schedules {
		ids[s.ID] = true
		studentIDs := strings.Join(s.StudentIDs, ";")
		hash := rowHash(s.ID, s.DayOfWeek, s.StartTime, s.EndTime, s.TeacherID, s.RoomID, s.Subject, studentIDs, s.EffectiveFrom, s.EffectiveUntil)
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO schedules (id, day_of_week, start_time, end_time, teacher_id, room_id, subject, student_ids, effective_from, effective_until, row_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID, s.DayOfWeek, s.StartTime, s.EndTime, s.TeacherID, s.RoomID, s.Subject, studentIDs, s.EffectiveFrom, s.EffectiveUntil, hash,
		)
		if err != nil {
			return err
		}
	}
	if err := deleteRemovedRows(tx, "schedules", ids); err != nil {
		return err
	}
	return tx.Commit()
}

// deleteRemovedRows removes rows from the table whose IDs are not in the provided set.
func deleteRemovedRows(tx *sql.Tx, table string, ids map[string]bool) error {
	if len(ids) == 0 {
		// If no data provided, clear the table
		_, err := tx.Exec("DELETE FROM " + table)
		return err
	}

	rows, err := tx.Query("SELECT id FROM " + table)
	if err != nil {
		return err
	}
	defer rows.Close()

	var toDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if !ids[id] {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE id = ?", id); err != nil {
			return err
		}
	}
	return nil
}
