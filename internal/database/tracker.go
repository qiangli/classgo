package database

import (
	"database/sql"
	"strings"
	"time"

	"classgo/internal/models"
)

const trackerItemCols = `id, name, COALESCE(notes,''), COALESCE(start_date,''), COALESCE(due_date,''),
	priority, recurrence, COALESCE(category,''), COALESCE(created_by,'admin'),
	active, deleted, COALESCE(created_at,''), COALESCE(updated_at,'')`

func scanTrackerItem(s interface{ Scan(...any) error }) (models.TrackerItem, error) {
	var it models.TrackerItem
	err := s.Scan(&it.ID, &it.Name, &it.Notes, &it.StartDate, &it.DueDate,
		&it.Priority, &it.Recurrence, &it.Category, &it.CreatedBy,
		&it.Active, &it.Deleted, &it.CreatedAt, &it.UpdatedAt)
	return it, err
}

// StudentItemCols is the column list for student_tracker_items queries.
const StudentItemCols = `id, student_id, name, COALESCE(notes,''), COALESCE(start_date,''), COALESCE(due_date,''),
	priority, recurrence, COALESCE(category,''), COALESCE(created_by,''), COALESCE(owner_type,'admin'),
	completed, COALESCE(completed_at,''), COALESCE(completed_by,''), requires_signoff,
	active, deleted, COALESCE(created_at,''), COALESCE(updated_at,'')`

// ScanStudentItemRow scans a single row into a StudentTrackerItem.
func ScanStudentItemRow(s interface{ Scan(...any) error }) (models.StudentTrackerItem, error) {
	return scanStudentItem(s)
}

func scanStudentItem(s interface{ Scan(...any) error }) (models.StudentTrackerItem, error) {
	var it models.StudentTrackerItem
	err := s.Scan(&it.ID, &it.StudentID, &it.Name, &it.Notes, &it.StartDate, &it.DueDate,
		&it.Priority, &it.Recurrence, &it.Category, &it.CreatedBy, &it.OwnerType,
		&it.Completed, &it.CompletedAt, &it.CompletedBy, &it.RequiresSignoff,
		&it.Active, &it.Deleted, &it.CreatedAt, &it.UpdatedAt)
	return it, err
}

// ListTrackerItems returns global tracker items.
func ListTrackerItems(db *sql.DB, includeDeleted bool) ([]models.TrackerItem, error) {
	q := "SELECT " + trackerItemCols + " FROM tracker_items"
	if !includeDeleted {
		q += " WHERE deleted = 0"
	}
	q += " ORDER BY priority = 'high' DESC, priority = 'medium' DESC, id"

	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TrackerItem
	for rows.Next() {
		it, err := scanTrackerItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SaveTrackerItem inserts or updates a global tracker item.
func SaveTrackerItem(db *sql.DB, item models.TrackerItem) (int64, error) {
	if item.ID > 0 {
		_, err := db.Exec(
			`UPDATE tracker_items SET name=?, notes=?, start_date=?, due_date=?,
			 priority=?, recurrence=?, category=?, active=?,
			 updated_at=datetime('now','localtime') WHERE id=?`,
			item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.DueDate),
			item.Priority, item.Recurrence, nullStr(item.Category), item.Active, item.ID,
		)
		return int64(item.ID), err
	}
	result, err := db.Exec(
		`INSERT INTO tracker_items (name, notes, start_date, due_date, priority, recurrence, category, created_by, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.DueDate),
		item.Priority, item.Recurrence, nullStr(item.Category), item.CreatedBy, item.Active,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// DeleteTrackerItem soft-deletes a global tracker item.
func DeleteTrackerItem(db *sql.DB, id int) error {
	_, err := db.Exec("UPDATE tracker_items SET deleted = 1 WHERE id = ?", id)
	return err
}

// ListStudentTrackerItems returns ad hoc tracker items for a specific student.
func ListStudentTrackerItems(db *sql.DB, studentID string) ([]models.StudentTrackerItem, error) {
	rows, err := db.Query(
		"SELECT "+StudentItemCols+" FROM student_tracker_items WHERE student_id = ? AND deleted = 0 ORDER BY priority = 'high' DESC, due_date, id",
		studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.StudentTrackerItem
	for rows.Next() {
		it, err := scanStudentItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ListStudentTrackerItemsByCreator returns items created by a specific user.
func ListStudentTrackerItemsByCreator(db *sql.DB, createdBy string) ([]models.StudentTrackerItem, error) {
	rows, err := db.Query(
		"SELECT "+StudentItemCols+" FROM student_tracker_items WHERE created_by = ? AND deleted = 0 ORDER BY student_id, due_date, id",
		createdBy,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.StudentTrackerItem
	for rows.Next() {
		it, err := scanStudentItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SaveStudentTrackerItem inserts or updates a per-student ad hoc tracker item.
func SaveStudentTrackerItem(db *sql.DB, item models.StudentTrackerItem) (int64, error) {
	if item.ID > 0 {
		_, err := db.Exec(
			`UPDATE student_tracker_items SET name=?, notes=?, start_date=?, due_date=?,
			 priority=?, recurrence=?, category=?, requires_signoff=?, active=?,
			 updated_at=datetime('now','localtime') WHERE id=?`,
			item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.DueDate),
			item.Priority, item.Recurrence, nullStr(item.Category), item.RequiresSignoff, item.Active, item.ID,
		)
		return int64(item.ID), err
	}
	result, err := db.Exec(
		`INSERT INTO student_tracker_items (student_id, name, notes, start_date, due_date,
		 priority, recurrence, category, created_by, owner_type, requires_signoff, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.StudentID, item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.DueDate),
		item.Priority, item.Recurrence, nullStr(item.Category), item.CreatedBy, item.OwnerType, item.RequiresSignoff, item.Active,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// DeleteStudentTrackerItem soft-deletes a per-student ad hoc tracker item.
func DeleteStudentTrackerItem(db *sql.DB, id int) error {
	_, err := db.Exec("UPDATE student_tracker_items SET deleted = 1 WHERE id = ?", id)
	return err
}

// CompleteStudentTrackerItem marks a one-time task as completed.
func CompleteStudentTrackerItem(db *sql.DB, id int, completedBy string) error {
	_, err := db.Exec(
		"UPDATE student_tracker_items SET completed = 1, completed_at = datetime('now','localtime'), completed_by = ? WHERE id = ?",
		completedBy, id,
	)
	return err
}

// UncompleteStudentTrackerItem marks a completed task as not completed.
func UncompleteStudentTrackerItem(db *sql.DB, id int) error {
	_, err := db.Exec(
		"UPDATE student_tracker_items SET completed = 0, completed_at = NULL, completed_by = NULL WHERE id = ?",
		id,
	)
	return err
}

// PendingSignoffItems returns due items that require signoff and haven't been responded to today.
// Used by checkout to block until the student signs off on required tasks.
func PendingSignoffItems(db *sql.DB, studentID string) ([]models.DueItem, error) {
	today := time.Now().Format("2006-01-02")
	allDue, err := GetDueItems(db, studentID, today)
	if err != nil {
		return nil, err
	}

	// Filter: only adhoc items with requires_signoff=true
	var pending []models.DueItem
	for _, it := range allDue {
		if it.ItemType == "adhoc" {
			// Check requires_signoff flag
			var reqSignoff bool
			db.QueryRow("SELECT requires_signoff FROM student_tracker_items WHERE id = ? AND deleted = 0", it.ItemID).Scan(&reqSignoff)
			if reqSignoff {
				pending = append(pending, it)
			}
		}
	}
	return pending, nil
}

// GetDueItems returns items due today for a student, respecting dates and recurrence.
// Recurrence logic:
//   - daily: due every day, check if responded today
//   - weekly: due once per week, check if responded this week (Mon-Sun)
//   - monthly: due once per month, check if responded this month
//   - none (one-time): due until completed or past due_date
func GetDueItems(db *sql.DB, studentID string, date string) ([]models.DueItem, error) {
	// Parse date for period calculations
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		t = time.Now()
		date = t.Format("2006-01-02")
	}
	weekStart := t.AddDate(0, 0, -int(t.Weekday()-time.Monday))
	if t.Weekday() == time.Sunday {
		weekStart = t.AddDate(0, 0, -6)
	}
	weekStartStr := weekStart.Format("2006-01-02")
	monthStart := t.Format("2006-01")

	rows, err := db.Query(`
		-- Global items (daily recurrence by default)
		SELECT 'global' AS item_type, ti.id AS item_id, ti.name, ti.priority, COALESCE(ti.category,''), COALESCE(ti.due_date,''), ti.recurrence
		FROM tracker_items ti
		WHERE ti.active = 1 AND ti.deleted = 0
		AND (ti.start_date IS NULL OR ti.start_date <= ?)
		AND (ti.due_date IS NULL OR ti.due_date >= ?)
		AND (
			(ti.recurrence = 'daily' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'global' AND tr.response_date = ?
			))
			OR (ti.recurrence = 'weekly' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'global' AND tr.response_date >= ?
			))
			OR (ti.recurrence = 'monthly' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'global' AND strftime('%Y-%m', tr.response_date) = ?
			))
			OR (ti.recurrence = 'none' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'global' AND tr.status = 'done'
			))
		)
		UNION ALL
		-- Student-specific items (one-time by default)
		SELECT 'adhoc' AS item_type, sti.id AS item_id, sti.name, sti.priority, COALESCE(sti.category,''), COALESCE(sti.due_date,''), sti.recurrence
		FROM student_tracker_items sti
		WHERE sti.student_id = ? AND sti.active = 1 AND sti.deleted = 0 AND sti.completed = 0
		AND (sti.start_date IS NULL OR sti.start_date <= ?)
		AND (sti.due_date IS NULL OR sti.due_date >= ?)
		AND (
			(sti.recurrence = 'daily' AND sti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'adhoc' AND tr.response_date = ?
			))
			OR (sti.recurrence = 'weekly' AND sti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'adhoc' AND tr.response_date >= ?
			))
			OR (sti.recurrence = 'monthly' AND sti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'adhoc' AND strftime('%Y-%m', tr.response_date) = ?
			))
			OR (sti.recurrence = 'none' AND sti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.item_type = 'adhoc' AND tr.status = 'done'
			))
		)`,
		// Global params
		date, date,
		studentID, date, // daily
		studentID, weekStartStr, // weekly
		studentID, monthStart, // monthly
		studentID, // none
		// Student params
		studentID, date, date,
		studentID, date, // daily
		studentID, weekStartStr, // weekly
		studentID, monthStart, // monthly
		studentID, // none
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.DueItem
	for rows.Next() {
		var it models.DueItem
		if err := rows.Scan(&it.ItemType, &it.ItemID, &it.Name, &it.Priority, &it.Category, &it.DueDate, &it.Recurrence); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SaveTrackerResponses saves responses and performs checkout in a single transaction.
func SaveTrackerResponses(db *sql.DB, studentID, studentName string, responses []models.TrackerResponse) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Perform checkout
	result, err := tx.Exec(
		"UPDATE attendance SET check_out_time = datetime('now','localtime') WHERE student_name = ? AND date(check_in_time) = date('now','localtime') AND check_out_time IS NULL",
		studentName,
	)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return 0, nil
	}

	// Get the attendance ID for linking
	var attendanceID int64
	err = tx.QueryRow(
		"SELECT id FROM attendance WHERE student_name = ? AND date(check_in_time) = date('now','localtime') ORDER BY check_in_time DESC LIMIT 1",
		studentName,
	).Scan(&attendanceID)
	if err != nil {
		return 0, err
	}

	// Insert responses
	stmt, err := tx.Prepare(
		"INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, notes, attendance_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, r := range responses {
		_, err = stmt.Exec(studentID, studentName, r.ItemType, r.ItemID, r.ItemName, r.Status, r.Notes, attendanceID)
		if err != nil {
			return 0, err
		}
	}

	return rows, tx.Commit()
}

// GetTrackerResponsesForDate returns all tracker responses for a student on a given date.
func GetTrackerResponsesForDate(db *sql.DB, studentID, date string) ([]models.TrackerResponse, error) {
	rows, err := db.Query(
		`SELECT id, student_id, student_name, item_type, item_id, item_name, status,
		        COALESCE(notes,''), response_date, COALESCE(attendance_id, 0), COALESCE(responded_at, '')
		 FROM tracker_responses WHERE student_id = ? AND response_date = ? ORDER BY id`,
		studentID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []models.TrackerResponse
	for rows.Next() {
		var r models.TrackerResponse
		if err := rows.Scan(&r.ID, &r.StudentID, &r.StudentName, &r.ItemType, &r.ItemID, &r.ItemName, &r.Status, &r.Notes, &r.ResponseDate, &r.AttendanceID, &r.RespondedAt); err != nil {
			return nil, err
		}
		responses = append(responses, r)
	}
	return responses, rows.Err()
}

// GetProgressStats returns completion statistics for students over a date range.
func GetProgressStats(db *sql.DB, studentIDs []string, startDate, endDate string) ([]models.ProgressStats, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(studentIDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(studentIDs)+2)
	for _, id := range studentIDs {
		args = append(args, id)
	}
	args = append(args, startDate, endDate)

	rows, err := db.Query(`
		SELECT student_id, student_name,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END) as done_count,
			SUM(CASE WHEN status = 'not_done' THEN 1 ELSE 0 END) as not_done_count
		FROM tracker_responses
		WHERE student_id IN (`+placeholders+`)
		AND response_date >= ? AND response_date <= ?
		GROUP BY student_id, student_name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.ProgressStats
	for rows.Next() {
		var s models.ProgressStats
		if err := rows.Scan(&s.StudentID, &s.StudentName, &s.TotalItems, &s.DoneCount, &s.NotDone); err != nil {
			return nil, err
		}
		if s.TotalItems > 0 {
			s.Completion = float64(s.DoneCount) / float64(s.TotalItems) * 100
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// BulkCreateStudentItems creates the same tracker item for multiple students.
func BulkCreateStudentItems(db *sql.DB, studentIDs []string, item models.StudentTrackerItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO student_tracker_items (student_id, name, notes, start_date, due_date,
		 priority, recurrence, category, created_by, owner_type, requires_signoff, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sid := range studentIDs {
		_, err = stmt.Exec(sid, item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.DueDate),
			item.Priority, item.Recurrence, nullStr(item.Category), item.CreatedBy, item.OwnerType, item.RequiresSignoff, item.Active)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetTeacherStudentIDs returns all student IDs assigned to a teacher through schedules.
func GetTeacherStudentIDs(db *sql.DB, teacherID string) ([]string, error) {
	rows, err := db.Query(
		"SELECT DISTINCT student_ids FROM schedules WHERE teacher_id = ? AND deleted = 0",
		teacherID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var sids string
		if err := rows.Scan(&sids); err != nil {
			continue
		}
		for _, id := range splitIDs(sids) {
			seen[id] = true
		}
	}

	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetParentStudentIDs returns student IDs linked to a parent.
func GetParentStudentIDs(db *sql.DB, parentID string) ([]string, error) {
	rows, err := db.Query(
		"SELECT id FROM students WHERE parent_id = ? AND active = 1 AND deleted = 0",
		parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAllTasksForStudent returns both global and student-specific items for calendar/list views.
func GetAllTasksForStudent(db *sql.DB, studentID string) ([]models.DueItem, error) {
	rows, err := db.Query(`
		SELECT 'global', ti.id, ti.name, ti.priority, COALESCE(ti.category,''), COALESCE(ti.due_date,''), ti.recurrence
		FROM tracker_items ti WHERE ti.active = 1 AND ti.deleted = 0
		UNION ALL
		SELECT 'adhoc', sti.id, sti.name, sti.priority, COALESCE(sti.category,''), COALESCE(sti.due_date,''), sti.recurrence
		FROM student_tracker_items sti WHERE sti.student_id = ? AND sti.active = 1 AND sti.deleted = 0`,
		studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.DueItem
	for rows.Next() {
		var it models.DueItem
		if err := rows.Scan(&it.ItemType, &it.ItemID, &it.Name, &it.Priority, &it.Category, &it.DueDate, &it.Recurrence); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func splitIDs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	sep := ";"
	if !strings.Contains(s, ";") {
		sep = ","
	}
	var ids []string
	for _, id := range strings.Split(s, sep) {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetGlobalTrackerItems returns all active, non-deleted global tracker items.
func GetGlobalTrackerItems(db *sql.DB) ([]models.TrackerItem, error) {
	rows, err := db.Query(`SELECT ` + trackerItemCols + ` FROM tracker_items WHERE active = 1 AND deleted = 0 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.TrackerItem
	for rows.Next() {
		it, err := scanTrackerItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetLatestTrackerValues returns the latest tracker response notes per global item for a student.
func GetLatestTrackerValues(db *sql.DB, studentID string) (map[int]string, error) {
	rows, err := db.Query(`SELECT item_id, notes FROM tracker_responses
		WHERE student_id = ? AND item_type = 'global'
		AND id IN (SELECT MAX(id) FROM tracker_responses
		           WHERE student_id = ? AND item_type = 'global' GROUP BY item_id)`,
		studentID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int]string)
	for rows.Next() {
		var id int
		var notes string
		if err := rows.Scan(&id, &notes); err != nil {
			return nil, err
		}
		result[id] = notes
	}
	return result, rows.Err()
}

// SaveProfileTrackerValues saves tracker values from the profile form as tracker_response rows.
func SaveProfileTrackerValues(db *sql.DB, studentID, studentName string, values map[int]string) error {
	if len(values) == 0 {
		return nil
	}
	// Build item name lookup
	items, err := GetGlobalTrackerItems(db)
	if err != nil {
		return err
	}
	nameMap := make(map[int]string)
	for _, it := range items {
		nameMap[it.ID] = it.Name
	}

	today := time.Now().Format("2006-01-02")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for itemID, notes := range values {
		if notes == "" {
			continue
		}
		itemName := nameMap[itemID]
		if itemName == "" {
			continue
		}
		_, err := tx.Exec(`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, notes, response_date)
			VALUES (?, ?, 'global', ?, ?, 'done', ?, ?)`,
			studentID, studentName, itemID, itemName, notes, today)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AutoAssignProfileTasks creates student_tracker_items for global tracker items that the student
// has not yet responded to and does not already have assigned. Grade-aware filtering is applied.
func AutoAssignProfileTasks(db *sql.DB, studentID, grade string) error {
	items, err := GetGlobalTrackerItems(db)
	if err != nil {
		return err
	}

	// Get existing responses
	existingValues, err := GetLatestTrackerValues(db, studentID)
	if err != nil {
		return err
	}

	// Get existing student_tracker_items by name
	existingItems := make(map[string]bool)
	rows, err := db.Query(`SELECT name FROM student_tracker_items WHERE student_id = ? AND deleted = 0`, studentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		rows.Scan(&name)
		existingItems[name] = true
	}

	gradeNum := parseGradeNum(grade)

	for _, it := range items {
		// Skip if already has a value or assignment
		if existingValues[it.ID] != "" || existingItems[it.Name] {
			continue
		}
		// Grade-aware filtering
		if !shouldAssignForGrade(it.Name, it.Category, gradeNum) {
			continue
		}
		db.Exec(`INSERT INTO student_tracker_items (student_id, name, priority, recurrence, category, created_by, owner_type, requires_signoff, active)
			VALUES (?, ?, ?, 'none', ?, 'system', 'admin', 0, 1)`,
			studentID, it.Name, it.Priority, it.Category)
	}
	return nil
}

func parseGradeNum(grade string) int {
	n := 0
	for _, c := range grade {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func shouldAssignForGrade(name, category string, grade int) bool {
	if grade == 0 {
		return true // unknown grade, assign all
	}
	switch {
	case strings.Contains(name, "PSAT 8/9"):
		return grade <= 9
	case strings.Contains(name, "PSAT 10"):
		return grade >= 10
	case strings.Contains(name, "PSAT 11"), strings.Contains(name, "NMSQT"):
		return grade >= 11
	case category == "SAT":
		return grade >= 10
	case category == "AP":
		return grade >= 9
	}
	return true
}
