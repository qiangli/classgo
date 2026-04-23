package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"classgo/internal/models"
)

// taskItemCols is the column list for task_items queries.
const taskItemCols = `id, scope, COALESCE(schedule_id,''), COALESCE(student_id,''),
	COALESCE(type,'task'), name, COALESCE(notes,''),
	COALESCE(start_date,''), COALESCE(end_date,''), priority, recurrence, COALESCE(category,''),
	COALESCE(criteria,''), COALESCE(group_id,''), COALESCE(group_order,0),
	COALESCE(created_by,'admin'), COALESCE(owner_type,'admin'),
	completed, COALESCE(completed_at,''), COALESCE(completed_by,''),
	active, deleted, COALESCE(deleted_at,''), COALESCE(deleted_by,''),
	COALESCE(created_at,''), COALESCE(updated_at,'')`

func scanTaskItem(s interface{ Scan(...any) error }) (models.TaskItem, error) {
	var it models.TaskItem
	err := s.Scan(&it.ID, &it.Scope, &it.ScheduleID, &it.StudentID,
		&it.Type, &it.Name, &it.Notes,
		&it.StartDate, &it.EndDate, &it.Priority, &it.Recurrence, &it.Category,
		&it.Criteria, &it.GroupID, &it.GroupOrder,
		&it.CreatedBy, &it.OwnerType,
		&it.Completed, &it.CompletedAt, &it.CompletedBy,
		&it.Active, &it.Deleted, &it.DeletedAt, &it.DeletedBy,
		&it.CreatedAt, &it.UpdatedAt)
	return it, err
}

// scopeToItemType maps scope to the item_type string used in tracker_responses.
func scopeToItemType(scope int) string {
	switch scope {
	case models.ScopeCenter:
		return "global"
	case models.ScopeClass:
		return "class"
	case models.ScopePersonal:
		return "personal"
	}
	return "personal"
}

// itemTypeToScope maps an item_type string to a scope constant.
func itemTypeToScope(itemType string) int {
	switch itemType {
	case "global":
		return models.ScopeCenter
	case "class":
		return models.ScopeClass
	case "personal":
		return models.ScopePersonal
	}
	return models.ScopePersonal
}

// --- List functions ---

// ListTaskItems returns task items filtered by scope.
func ListTaskItems(db *sql.DB, scope int, includeDeleted bool) ([]models.TaskItem, error) {
	q := "SELECT " + taskItemCols + " FROM task_items WHERE scope = ?"
	if !includeDeleted {
		q += " AND deleted = 0"
	}
	q += " ORDER BY priority = 'high' DESC, priority = 'medium' DESC, id"

	rows, err := db.Query(q, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TaskItem
	for rows.Next() {
		it, err := scanTaskItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ListTrackerItems returns center-scoped (global) items. Backward-compat wrapper.
func ListTrackerItems(db *sql.DB, includeDeleted bool) ([]models.TaskItem, error) {
	return ListTaskItems(db, models.ScopeCenter, includeDeleted)
}

// ListStudentTrackerItems returns personal items for a specific student.
func ListStudentTrackerItems(db *sql.DB, studentID string) ([]models.TaskItem, error) {
	rows, err := db.Query(
		"SELECT "+taskItemCols+" FROM task_items WHERE scope = ? AND student_id = ? AND deleted = 0 ORDER BY priority = 'high' DESC, end_date, id",
		models.ScopePersonal, studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TaskItem
	for rows.Next() {
		it, err := scanTaskItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ListStudentTrackerItemsByCreator returns items created by a specific user of the given type.
func ListStudentTrackerItemsByCreator(db *sql.DB, createdBy, ownerType string) ([]models.TaskItem, error) {
	rows, err := db.Query(
		"SELECT "+taskItemCols+" FROM task_items WHERE created_by = ? AND owner_type = ? AND deleted = 0 ORDER BY student_id, end_date, id",
		createdBy, ownerType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TaskItem
	for rows.Next() {
		it, err := scanTaskItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// --- Save / Delete ---

// SaveTaskItem inserts or updates a task item.
func SaveTaskItem(db *sql.DB, item models.TaskItem) (int64, error) {
	if item.Type == "" {
		item.Type = models.TaskTypeTask
	}
	if item.ID > 0 {
		_, err := db.Exec(
			`UPDATE task_items SET type=?, name=?, notes=?, start_date=?, end_date=?,
			 priority=?, recurrence=?, category=?, criteria=?, group_id=?, group_order=?, active=?,
			 updated_at=datetime('now','localtime') WHERE id=?`,
			item.Type, item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.EndDate),
			item.Priority, item.Recurrence, nullStr(item.Category),
			nullStr(item.Criteria), nullStr(item.GroupID), nullInt(item.GroupOrder),
			item.Active, item.ID,
		)
		return int64(item.ID), err
	}
	result, err := db.Exec(
		`INSERT INTO task_items (scope, schedule_id, student_id, type, name, notes, start_date, end_date,
		 priority, recurrence, category, criteria, group_id, group_order, created_by, owner_type, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.Scope, nullStr(item.ScheduleID), nullStr(item.StudentID),
		item.Type, item.Name, item.Notes, nullStr(item.StartDate), nullStr(item.EndDate),
		item.Priority, item.Recurrence, nullStr(item.Category),
		nullStr(item.Criteria), nullStr(item.GroupID), nullInt(item.GroupOrder),
		item.CreatedBy, item.OwnerType, item.Active,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// SaveTrackerItem inserts or updates a center-scoped item. Backward-compat wrapper.
func SaveTrackerItem(db *sql.DB, item models.TaskItem) (int64, error) {
	item.Scope = models.ScopeCenter
	return SaveTaskItem(db, item)
}

// SaveStudentTrackerItem inserts or updates a personal-scoped item. Backward-compat wrapper.
func SaveStudentTrackerItem(db *sql.DB, item models.TaskItem) (int64, error) {
	item.Scope = models.ScopePersonal
	return SaveTaskItem(db, item)
}

// DeleteTaskItem soft-deletes a task item with audit info.
func DeleteTaskItem(db *sql.DB, id int, deletedBy string) error {
	_, err := db.Exec("UPDATE task_items SET deleted = 1, deleted_at = datetime('now','localtime'), deleted_by = ? WHERE id = ?", deletedBy, id)
	return err
}

// DeleteTrackerItem soft-deletes a task item. Backward-compat wrapper.
func DeleteTrackerItem(db *sql.DB, id int, deletedBy string) error {
	return DeleteTaskItem(db, id, deletedBy)
}

// DeleteStudentTrackerItem soft-deletes a task item. Backward-compat wrapper.
func DeleteStudentTrackerItem(db *sql.DB, id int, deletedBy string) error {
	return DeleteTaskItem(db, id, deletedBy)
}

// --- Complete / Uncomplete ---

// CompleteTaskItem marks a one-time task as completed and records a tracker_response.
func CompleteTaskItem(db *sql.DB, id int, completedBy string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"UPDATE task_items SET completed = 1, completed_at = datetime('now','localtime'), completed_by = ? WHERE id = ?",
		completedBy, id,
	); err != nil {
		return err
	}

	var studentID, itemName string
	var scope int
	if err := tx.QueryRow("SELECT student_id, name, scope FROM task_items WHERE id = ?", id).Scan(&studentID, &itemName, &scope); err != nil {
		return err
	}
	var studentName string
	tx.QueryRow("SELECT COALESCE(first_name,'')||' '||COALESCE(last_name,'') FROM students WHERE id = ?", studentID).Scan(&studentName)

	if _, err := tx.Exec(
		"INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, attendance_id) VALUES (?, ?, ?, ?, ?, 'done', 0)",
		studentID, strings.TrimSpace(studentName), scopeToItemType(scope), id, itemName,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// CompleteStudentTrackerItem is a backward-compat wrapper.
func CompleteStudentTrackerItem(db *sql.DB, id int, completedBy string) error {
	return CompleteTaskItem(db, id, completedBy)
}

// UncompleteTaskItem marks a completed task as not completed and removes dashboard-created responses.
func UncompleteTaskItem(db *sql.DB, id int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"UPDATE task_items SET completed = 0, completed_at = NULL, completed_by = NULL WHERE id = ?",
		id,
	); err != nil {
		return err
	}
	// Only remove dashboard-created responses (attendance_id=0), not checkout responses
	if _, err := tx.Exec(
		"DELETE FROM tracker_responses WHERE item_id = ? AND status = 'done' AND attendance_id = 0",
		id,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// UncompleteStudentTrackerItem is a backward-compat wrapper.
func UncompleteStudentTrackerItem(db *sql.DB, id int) error {
	return UncompleteTaskItem(db, id)
}

// --- Criteria Matching ---

// StudentAttributes holds student fields used for criteria filtering.
type StudentAttributes struct {
	Grade         int
	Birthplace    string
	FirstLanguage string
	School        string
}

// Criteria is the JSON structure for attribute-based filtering on scope=1 items.
type Criteria struct {
	GradeMin      *int     `json:"grade_min,omitempty"`
	GradeMax      *int     `json:"grade_max,omitempty"`
	Birthplace    []string `json:"birthplace,omitempty"`
	FirstLanguage []string `json:"first_language,omitempty"`
	School        []string `json:"school,omitempty"`
}

// loadStudentAttributes loads the fields needed for criteria matching.
func loadStudentAttributes(db *sql.DB, studentID string) StudentAttributes {
	var grade, birthplace, firstLang, school sql.NullString
	db.QueryRow(`SELECT COALESCE(grade,''), COALESCE(birthplace,''), COALESCE(first_language,''), COALESCE(school,'')
		FROM students WHERE id = ?`, studentID).Scan(&grade, &birthplace, &firstLang, &school)
	return StudentAttributes{
		Grade:         parseGradeNum(grade.String),
		Birthplace:    birthplace.String,
		FirstLanguage: firstLang.String,
		School:        school.String,
	}
}

// matchesCriteria returns true if the student matches the criteria JSON.
// Empty or invalid criteria matches all students.
func matchesCriteria(criteriaJSON string, student StudentAttributes) bool {
	if criteriaJSON == "" {
		return true
	}
	var c Criteria
	if err := json.Unmarshal([]byte(criteriaJSON), &c); err != nil {
		return true // invalid JSON matches all
	}
	if c.GradeMin != nil && student.Grade > 0 && student.Grade < *c.GradeMin {
		return false
	}
	if c.GradeMax != nil && student.Grade > 0 && student.Grade > *c.GradeMax {
		return false
	}
	if len(c.Birthplace) > 0 && student.Birthplace != "" {
		if !containsIgnoreCase(c.Birthplace, student.Birthplace) {
			return false
		}
	}
	if len(c.FirstLanguage) > 0 && student.FirstLanguage != "" {
		if !containsIgnoreCase(c.FirstLanguage, student.FirstLanguage) {
			return false
		}
	}
	if len(c.School) > 0 && student.School != "" {
		if !containsIgnoreCase(c.School, student.School) {
			return false
		}
	}
	return true
}

func containsIgnoreCase(list []string, val string) bool {
	lower := strings.ToLower(val)
	for _, s := range list {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}

// --- Due Items ---

// PendingSignoffItems returns due items that require signoff for checkout blocking.
func PendingSignoffItems(db *sql.DB, studentID string) ([]models.DueItem, error) {
	today := time.Now().Format("2006-01-02")
	allDue, err := GetDueItems(db, studentID, today)
	if err != nil {
		return nil, err
	}

	var pending []models.DueItem
	for _, it := range allDue {
		if it.Type == models.TaskTypeTodo {
			pending = append(pending, it)
		}
	}
	return pending, nil
}

// GetDueItems returns items due today for a student, respecting scope, dates, and recurrence.
func GetDueItems(db *sql.DB, studentID string, date string) ([]models.DueItem, error) {
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

	// Load student attributes once for criteria filtering
	studentAttrs := loadStudentAttributes(db, studentID)

	rows, err := db.Query(`
		SELECT ti.scope, ti.id, COALESCE(ti.type,'task'), ti.name, ti.priority, COALESCE(ti.category,''),
		       COALESCE(ti.end_date,''), ti.recurrence,
		       COALESCE(ti.criteria,''), COALESCE(ti.group_id,''), COALESCE(ti.group_order,0)
		FROM task_items ti
		WHERE ti.active = 1 AND ti.deleted = 0
		AND (ti.start_date IS NULL OR ti.start_date <= ?)
		AND (ti.end_date IS NULL OR ti.end_date >= ?)
		AND (
			(ti.scope = 1)
			OR (ti.scope = 2 AND EXISTS (
				SELECT 1 FROM schedules s WHERE s.id = ti.schedule_id AND s.deleted = 0
				AND (';' || REPLACE(s.student_ids, ',', ';') || ';') LIKE ('%;' || ? || ';%')
			))
			OR (ti.scope = 3 AND ti.student_id = ? AND ti.completed = 0)
		)
		AND (
			(ti.recurrence = 'daily' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND COALESCE(tr.due_date, tr.response_date) = ?
			))
			OR (ti.recurrence = 'weekly' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND COALESCE(tr.due_date, tr.response_date) >= ?
			))
			OR (ti.recurrence = 'monthly' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND strftime('%Y-%m', COALESCE(tr.due_date, tr.response_date)) = ?
			))
			OR (ti.recurrence = 'none' AND ti.id NOT IN (
				SELECT tr.item_id FROM tracker_responses tr WHERE tr.student_id = ? AND tr.status = 'done'
			))
		)`,
		date, date,
		studentID,       // scope=2 schedule enrollment check
		studentID,       // scope=3 student_id check
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
		var criteriaJSON string
		if err := rows.Scan(&it.Scope, &it.ItemID, &it.Type, &it.Name, &it.Priority, &it.Category, &it.EndDate, &it.Recurrence, &criteriaJSON, &it.GroupID, &it.GroupOrder); err != nil {
			return nil, err
		}
		// Filter scope=1 items by criteria
		if it.Scope == models.ScopeCenter && !matchesCriteria(criteriaJSON, studentAttrs) {
			continue
		}
		it.ItemType = scopeToItemType(it.Scope)
		it.RequiresSignoff = it.Type == models.TaskTypeTodo
		items = append(items, it)
	}
	return items, rows.Err()
}

// --- Responses ---

// SaveTrackerResponses saves responses and performs checkout in a single transaction.
func SaveTrackerResponses(db *sql.DB, studentID, studentName string, responses []models.TrackerResponse) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"UPDATE attendance SET check_out_time = datetime('now','localtime') WHERE student_name = ? AND date(check_in_time) = date('now','localtime') AND check_out_time IS NULL",
		studentName,
	)
	if err != nil {
		return 0, err
	}
	affectedRows, _ := result.RowsAffected()
	if affectedRows == 0 {
		return 0, nil
	}

	var attendanceID int64
	err = tx.QueryRow(
		"SELECT id FROM attendance WHERE student_name = ? AND date(check_in_time) = date('now','localtime') ORDER BY check_in_time DESC LIMIT 1",
		studentName,
	).Scan(&attendanceID)
	if err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(
		"INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, notes, attendance_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, r := range responses {
		itemType := r.ItemType
		if itemType == "" {
			// Derive item_type from task_items.scope if not provided
			var scope int
			if tx.QueryRow("SELECT scope FROM task_items WHERE id = ?", r.ItemID).Scan(&scope) == nil {
				itemType = scopeToItemType(scope)
			} else {
				itemType = "personal"
			}
		}
		_, err = stmt.Exec(studentID, studentName, itemType, r.ItemID, r.ItemName, r.Status, r.Notes, attendanceID)
		if err != nil {
			return 0, err
		}
	}

	return affectedRows, tx.Commit()
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

// SaveLateSignoff records a late signoff for a task item.
func SaveLateSignoff(db *sql.DB, studentID, dueDate string, itemID int, status, notes string) error {
	var studentName string
	db.QueryRow("SELECT COALESCE(first_name,'')||' '||COALESCE(last_name,'') FROM students WHERE id = ?", studentID).Scan(&studentName)
	studentName = strings.TrimSpace(studentName)

	var itemName string
	var scope int
	db.QueryRow("SELECT name, scope FROM task_items WHERE id = ?", itemID).Scan(&itemName, &scope)

	_, err := db.Exec(
		`INSERT INTO tracker_responses (student_id, student_name, item_type, item_id, item_name, status, notes, due_date, is_late, attendance_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 0)`,
		studentID, studentName, scopeToItemType(scope), itemID, itemName, status, notes, dueDate,
	)
	return err
}

// --- Progress ---

type trackerItemDates struct {
	StartDate  string
	EndDate    string
	Recurrence string
}

func isActiveOnDate(it trackerItemDates, d time.Time) bool {
	ds := d.Format("2006-01-02")
	if it.StartDate != "" && it.StartDate > ds {
		return false
	}
	if it.EndDate != "" && it.EndDate < ds {
		return false
	}
	return true
}

func countExpectedForItem(it trackerItemDates, rangeStart, rangeEnd time.Time) int {
	switch it.Recurrence {
	case "daily":
		count := 0
		for d := rangeStart; !d.After(rangeEnd); d = d.AddDate(0, 0, 1) {
			if isActiveOnDate(it, d) {
				count++
			}
		}
		return count
	case "weekly":
		count := 0
		seen := make(map[string]bool)
		for d := rangeStart; !d.After(rangeEnd); d = d.AddDate(0, 0, 1) {
			yr, wk := d.ISOWeek()
			key := fmt.Sprintf("%d-%d", yr, wk)
			if !seen[key] && isActiveOnDate(it, d) {
				seen[key] = true
				count++
			}
		}
		return count
	case "monthly":
		count := 0
		seen := make(map[string]bool)
		for d := rangeStart; !d.After(rangeEnd); d = d.AddDate(0, 0, 1) {
			key := d.Format("2006-01")
			if !seen[key] && isActiveOnDate(it, d) {
				seen[key] = true
				count++
			}
		}
		return count
	case "none":
		for d := rangeStart; !d.After(rangeEnd); d = d.AddDate(0, 0, 1) {
			if isActiveOnDate(it, d) {
				return 1
			}
		}
		return 0
	default:
		return 0
	}
}

// GetProgressStats returns expected-based completion statistics for students over a date range.
func GetProgressStats(db *sql.DB, studentIDs []string, startDate, endDate string) ([]models.ProgressStats, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	rangeStart, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	rangeEnd, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	// Get all active center-scoped items (same expected count for all students)
	globalRows, err := db.Query(`
		SELECT COALESCE(start_date,''), COALESCE(end_date,''), recurrence
		FROM task_items WHERE scope = 1 AND active = 1 AND deleted = 0 AND type = 'todo'`)
	if err != nil {
		return nil, err
	}
	var globalItems []trackerItemDates
	for globalRows.Next() {
		var it trackerItemDates
		if err := globalRows.Scan(&it.StartDate, &it.EndDate, &it.Recurrence); err != nil {
			globalRows.Close()
			return nil, err
		}
		globalItems = append(globalItems, it)
	}
	globalRows.Close()

	globalExpected := 0
	for _, it := range globalItems {
		globalExpected += countExpectedForItem(it, rangeStart, rangeEnd)
	}

	// Student name lookup
	placeholders := strings.Repeat("?,", len(studentIDs))
	placeholders = placeholders[:len(placeholders)-1]

	nameArgs := make([]any, len(studentIDs))
	for i, id := range studentIDs {
		nameArgs[i] = id
	}
	nameMap := make(map[string]string)
	nameRows, err := db.Query(
		`SELECT id, COALESCE(first_name,'')||' '||COALESCE(last_name,'') FROM students WHERE id IN (`+placeholders+`)`,
		nameArgs...)
	if err == nil {
		for nameRows.Next() {
			var id, name string
			nameRows.Scan(&id, &name)
			nameMap[id] = strings.TrimSpace(name)
		}
		nameRows.Close()
	}

	// Done counts per student from tracker_responses
	doneArgs := make([]any, 0, len(studentIDs)+2)
	for _, id := range studentIDs {
		doneArgs = append(doneArgs, id)
	}
	doneArgs = append(doneArgs, startDate, endDate)
	doneRows, err := db.Query(`
		SELECT tr.student_id, COALESCE(tr.student_name,''),
			SUM(CASE WHEN tr.status = 'done' THEN 1 ELSE 0 END) as done_count
		FROM tracker_responses tr
		LEFT JOIN task_items ti ON tr.item_id = ti.id
		WHERE tr.student_id IN (`+placeholders+`)
		AND tr.response_date >= ? AND tr.response_date <= ?
		AND COALESCE(ti.type, 'task') = 'todo'
		GROUP BY tr.student_id`,
		doneArgs...,
	)
	if err != nil {
		return nil, err
	}
	doneMap := make(map[string]int)
	respNameMap := make(map[string]string)
	for doneRows.Next() {
		var sid, sname string
		var done int
		if err := doneRows.Scan(&sid, &sname, &done); err != nil {
			doneRows.Close()
			return nil, err
		}
		doneMap[sid] = done
		if sname != "" {
			respNameMap[sid] = sname
		}
	}
	doneRows.Close()

	// Build stats per student
	var stats []models.ProgressStats
	for _, sid := range studentIDs {
		// Student-specific items expected count (scope=3 personal items)
		studentExpected := 0
		stiRows, err := db.Query(`
			SELECT COALESCE(start_date,''), COALESCE(end_date,''), recurrence
			FROM task_items
			WHERE scope = 3 AND student_id = ? AND active = 1 AND deleted = 0 AND type = 'todo'`, sid)
		if err != nil {
			return nil, err
		}
		for stiRows.Next() {
			var it trackerItemDates
			if err := stiRows.Scan(&it.StartDate, &it.EndDate, &it.Recurrence); err != nil {
				stiRows.Close()
				return nil, err
			}
			studentExpected += countExpectedForItem(it, rangeStart, rangeEnd)
		}
		stiRows.Close()

		// TODO: Add scope=2 class items expected count (for enrolled students)

		total := globalExpected + studentExpected
		done := doneMap[sid]
		if done > total {
			done = total
		}
		name := nameMap[sid]
		if name == "" {
			name = respNameMap[sid]
		}

		s := models.ProgressStats{
			StudentID:   sid,
			StudentName: name,
			TotalItems:  total,
			DoneCount:   done,
			NotDone:     total - done,
		}
		if total > 0 {
			s.Completion = float64(done) / float64(total) * 100
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// --- All tasks for calendar/list ---

// GetAllTasksForStudent returns all active items visible to a student (all scopes).
func GetAllTasksForStudent(db *sql.DB, studentID string) ([]models.DueItem, error) {
	rows, err := db.Query(`
		SELECT ti.scope, ti.id, COALESCE(ti.type,'task'), ti.name, ti.priority, COALESCE(ti.category,''),
		       COALESCE(ti.end_date,''), ti.recurrence,
		       COALESCE(ti.group_id,''), COALESCE(ti.group_order,0)
		FROM task_items ti
		WHERE ti.active = 1 AND ti.deleted = 0
		AND (
			(ti.scope = 1)
			OR (ti.scope = 2 AND EXISTS (
				SELECT 1 FROM schedules s WHERE s.id = ti.schedule_id AND s.deleted = 0
				AND (';' || REPLACE(s.student_ids, ',', ';') || ';') LIKE ('%;' || ? || ';%')
			))
			OR (ti.scope = 3 AND ti.student_id = ?)
		)`,
		studentID, studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.DueItem
	for rows.Next() {
		var it models.DueItem
		if err := rows.Scan(&it.Scope, &it.ItemID, &it.Type, &it.Name, &it.Priority, &it.Category, &it.EndDate, &it.Recurrence, &it.GroupID, &it.GroupOrder); err != nil {
			return nil, err
		}
		it.ItemType = scopeToItemType(it.Scope)
		it.RequiresSignoff = it.Type == models.TaskTypeTodo
		items = append(items, it)
	}
	return items, rows.Err()
}

// --- Student/Teacher/Parent ID lookups (unchanged) ---

// GetAllActiveStudentIDs returns all active student IDs.
func GetAllActiveStudentIDs(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT id FROM students WHERE deleted = 0 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetStudentIDForItem returns the student_id for a task item.
func GetStudentIDForItem(db *sql.DB, itemID int) (string, error) {
	var studentID string
	err := db.QueryRow("SELECT COALESCE(student_id,'') FROM task_items WHERE id = ?", itemID).Scan(&studentID)
	return studentID, err
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

// --- Bulk operations ---

// BulkCreateTaskItems creates the same task item for multiple students (scope=3).
func BulkCreateTaskItems(db *sql.DB, studentIDs []string, item models.TaskItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	itemType := item.Type
	if itemType == "" {
		itemType = models.TaskTypeTask
	}
	stmt, err := tx.Prepare(
		`INSERT INTO task_items (scope, student_id, type, name, notes, start_date, end_date,
		 priority, recurrence, category, created_by, owner_type, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sid := range studentIDs {
		_, err = stmt.Exec(models.ScopePersonal, sid, itemType, item.Name, item.Notes,
			nullStr(item.StartDate), nullStr(item.EndDate),
			item.Priority, item.Recurrence, nullStr(item.Category),
			item.CreatedBy, item.OwnerType, item.Active)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BulkCreateStudentItems is a backward-compat wrapper.
func BulkCreateStudentItems(db *sql.DB, studentIDs []string, item models.TaskItem) error {
	return BulkCreateTaskItems(db, studentIDs, item)
}

// --- Profile tracker values ---

// GetCenterTaskItems returns all active, non-deleted center-scoped items.
func GetCenterTaskItems(db *sql.DB) ([]models.TaskItem, error) {
	rows, err := db.Query("SELECT " + taskItemCols + " FROM task_items WHERE scope = 1 AND active = 1 AND deleted = 0 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.TaskItem
	for rows.Next() {
		it, err := scanTaskItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetGlobalTrackerItems is a backward-compat wrapper.
func GetGlobalTrackerItems(db *sql.DB) ([]models.TaskItem, error) {
	return GetCenterTaskItems(db)
}

// GetLatestTrackerValues returns the latest tracker response notes per center item for a student.
func GetLatestTrackerValues(db *sql.DB, studentID string) (map[int]string, error) {
	rows, err := db.Query(`SELECT tr.item_id, tr.notes FROM tracker_responses tr
		INNER JOIN task_items ti ON ti.id = tr.item_id AND ti.scope = 1
		WHERE tr.student_id = ?
		AND tr.id IN (SELECT MAX(id) FROM tracker_responses
		              WHERE student_id = ? GROUP BY item_id)`,
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
	items, err := GetCenterTaskItems(db)
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

// --- Auto-assign ---

// AutoAssignProfileTasks creates personal task items for center items that the student
// has not yet responded to. Grade-aware filtering is applied.
func AutoAssignProfileTasks(db *sql.DB, studentID, grade string) error {
	items, err := GetCenterTaskItems(db)
	if err != nil {
		return err
	}

	existingValues, err := GetLatestTrackerValues(db, studentID)
	if err != nil {
		return err
	}

	// Get existing personal items by name for this student
	existingItems := make(map[string]bool)
	rows, err := db.Query(`SELECT name FROM task_items WHERE scope = 3 AND student_id = ? AND deleted = 0`, studentID)
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
		if existingValues[it.ID] != "" || existingItems[it.Name] {
			continue
		}
		// Use criteria if set, otherwise fall back to grade-based rules
		studentAttrs := StudentAttributes{Grade: gradeNum}
		if it.Criteria != "" {
			if !matchesCriteria(it.Criteria, studentAttrs) {
				continue
			}
		} else if !shouldAssignForGrade(it.Name, it.Category, gradeNum) {
			continue
		}
		db.Exec(`INSERT INTO task_items (scope, student_id, type, name, priority, recurrence, category, created_by, owner_type, active)
			VALUES (3, ?, 'task', ?, ?, 'none', ?, 'system', 'admin', 1)`,
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
		return true
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

// --- Helpers ---

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

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

// Backward-compat exports used by handlers
var StudentItemCols = taskItemCols

func ScanStudentItemRow(s interface{ Scan(...any) error }) (models.TaskItem, error) {
	return scanTaskItem(s)
}
