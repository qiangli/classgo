package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/models"
)

// HandleDashboard serves the role-based dashboard page.
func (a *App) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	name, _ := a.lookupEntity(sess.EntityID)
	if name == "" {
		name = sess.Username
	}

	data := models.DashboardData{
		AppName:  a.AppName,
		UserType: sess.UserType,
		EntityID: sess.EntityID,
		UserName: name,
		Date:     time.Now().Format("Monday, January 2, 2006"),
		Accounts: a.GetAccountInfo(r),
	}

	a.Tmpl.ExecuteTemplate(w, "dashboard.html", data)
}

// HandleDashboardMyClasses returns schedule data for the logged-in teacher.
func (a *App) HandleDashboardMyClasses(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	teacherID := sess.EntityID
	if sess.UserType != "teacher" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "Teachers only"})
		return
	}

	rows, err := a.DB.Query(
		`SELECT id, day_of_week, start_time, end_time, COALESCE(room_id,''), COALESCE(subject,''), COALESCE(student_ids,''),
		 COALESCE(effective_from,''), COALESCE(effective_until,'')
		 FROM schedules WHERE teacher_id = ? AND deleted = 0 ORDER BY
		 CASE day_of_week WHEN 'Monday' THEN 1 WHEN 'Tuesday' THEN 2 WHEN 'Wednesday' THEN 3
		 WHEN 'Thursday' THEN 4 WHEN 'Friday' THEN 5 WHEN 'Saturday' THEN 6 WHEN 'Sunday' THEN 7 END,
		 start_time`,
		teacherID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	defer rows.Close()

	type classInfo struct {
		ID             string   `json:"id"`
		DayOfWeek      string   `json:"day_of_week"`
		StartTime      string   `json:"start_time"`
		EndTime        string   `json:"end_time"`
		RoomID         string   `json:"room_id"`
		RoomName       string   `json:"room_name"`
		Subject        string   `json:"subject"`
		StudentIDs     []string `json:"student_ids"`
		Students       []any    `json:"students"`
		EffectiveFrom  string   `json:"effective_from"`
		EffectiveUntil string   `json:"effective_until"`
	}

	var classes []classInfo
	for rows.Next() {
		var c classInfo
		var studentIDsStr string
		if err := rows.Scan(&c.ID, &c.DayOfWeek, &c.StartTime, &c.EndTime, &c.RoomID, &c.Subject, &studentIDsStr, &c.EffectiveFrom, &c.EffectiveUntil); err != nil {
			continue
		}
		c.StudentIDs = splitSemicolon(studentIDsStr)

		// Resolve room name
		if c.RoomID != "" {
			var rname string
			if err := a.DB.QueryRow("SELECT name FROM rooms WHERE id = ?", c.RoomID).Scan(&rname); err == nil {
				c.RoomName = rname
			}
		}

		// Resolve student names
		for _, sid := range c.StudentIDs {
			var fn, ln string
			if err := a.DB.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", sid).Scan(&fn, &ln); err == nil {
				c.Students = append(c.Students, map[string]string{"id": sid, "name": fn + " " + ln})
			}
		}

		classes = append(classes, c)
	}
	if classes == nil {
		classes = []classInfo{}
	}
	writeJSON(w, http.StatusOK, classes)
}

// HandleDashboardMyStudents returns student+parent info for the logged-in teacher or parent.
func (a *App) HandleDashboardMyStudents(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	var studentIDs []string
	switch sess.UserType {
	case "teacher":
		ids, _ := database.GetTeacherStudentIDs(a.DB, sess.EntityID)
		studentIDs = ids
	case "parent":
		ids, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
		studentIDs = ids
	case "student":
		studentIDs = []string{sess.EntityID}
	default:
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type studentInfo struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Grade     string `json:"grade"`
		School    string `json:"school"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		ParentID  string `json:"parent_id"`
		Parent    any    `json:"parent"`
	}

	var students []studentInfo
	for _, sid := range studentIDs {
		var s studentInfo
		var grade, school, email, phone, parentID sql.NullString
		err := a.DB.QueryRow(
			"SELECT id, first_name, last_name, grade, school, email, phone, parent_id FROM students WHERE id = ? AND active = 1",
			sid,
		).Scan(&s.ID, &s.FirstName, &s.LastName, &grade, &school, &email, &phone, &parentID)
		if err != nil {
			continue
		}
		s.Grade = grade.String
		s.School = school.String
		s.Email = email.String
		s.Phone = phone.String
		s.ParentID = parentID.String

		if s.ParentID != "" {
			var pfn, pln, pem, pph string
			if err := a.DB.QueryRow(
				"SELECT first_name, last_name, COALESCE(email,''), COALESCE(phone,'') FROM parents WHERE id = ?",
				s.ParentID,
			).Scan(&pfn, &pln, &pem, &pph); err == nil {
				s.Parent = map[string]string{"id": s.ParentID, "name": pfn + " " + pln, "email": pem, "phone": pph}
			}
		}
		students = append(students, s)
	}
	if students == nil {
		students = []studentInfo{}
	}
	writeJSON(w, http.StatusOK, students)
}

// HandleDashboardAllTasks returns all tasks for the logged-in user's students.
func (a *App) HandleDashboardAllTasks(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	studentID := r.URL.Query().Get("student_id")
	if studentID == "" {
		switch sess.UserType {
		case "student":
			studentID = sess.EntityID
		case "parent":
			ids, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
			if len(ids) > 0 {
				studentID = ids[0]
			}
		}
	}

	if studentID == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	items, err := database.GetAllTasksForStudent(a.DB, studentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if items == nil {
		items = []models.DueItem{}
	}

	// Also include student-specific items with full details
	studentItems, err := database.ListStudentTrackerItems(a.DB, studentID)
	if err != nil {
		studentItems = []models.StudentTrackerItem{}
	}
	if studentItems == nil {
		studentItems = []models.StudentTrackerItem{}
	}

	globalItems, err := database.ListTrackerItems(a.DB, false)
	if err != nil {
		globalItems = []models.TrackerItem{}
	}
	if globalItems == nil {
		globalItems = []models.TrackerItem{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"due_items":     items,
		"student_items": studentItems,
		"global_items":  globalItems,
	})
}

// HandleDashboardTeacherItems returns items created by the logged-in teacher.
func (a *App) HandleDashboardTeacherItems(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	items, err := database.ListStudentTrackerItemsByCreator(a.DB, sess.EntityID, sess.UserType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if items == nil {
		items = []models.StudentTrackerItem{}
	}

	// Resolve student names
	type itemWithName struct {
		models.StudentTrackerItem
		StudentName string `json:"student_name"`
	}
	var result []itemWithName
	for _, it := range items {
		name := a.lookupStudentName(it.StudentID)
		result = append(result, itemWithName{it, name})
	}
	if result == nil {
		result = []itemWithName{}
	}
	writeJSON(w, http.StatusOK, result)
}

// ==================== Classes & Enrollment ====================

// HandleDashboardClasses returns class listings for all roles.
// Students see all active schedules with enrollment status.
// Parents accept ?student_id= to check enrollment for a specific child.
// Teachers see only their own schedules.
func (a *App) HandleDashboardClasses(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	// Determine which student to check enrollment for
	checkStudentID := ""
	switch sess.UserType {
	case "student":
		checkStudentID = sess.EntityID
	case "parent":
		checkStudentID = r.URL.Query().Get("student_id")
		if checkStudentID != "" {
			// Verify this child belongs to the parent
			childIDs, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
			found := false
			for _, cid := range childIDs {
				if cid == checkStudentID {
					found = true
					break
				}
			}
			if !found {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "Not your child"})
				return
			}
		}
	}

	// Build query: teachers see only their own; others see all active schedules
	var query string
	var args []any
	if sess.UserType == "teacher" {
		query = `SELECT id, day_of_week, start_time, end_time, COALESCE(teacher_id,''), COALESCE(room_id,''),
			COALESCE(subject,''), COALESCE(student_ids,''), COALESCE(effective_from,''), COALESCE(effective_until,'')
			FROM schedules WHERE teacher_id = ? AND deleted = 0
			ORDER BY CASE day_of_week WHEN 'Monday' THEN 1 WHEN 'Tuesday' THEN 2 WHEN 'Wednesday' THEN 3
			WHEN 'Thursday' THEN 4 WHEN 'Friday' THEN 5 WHEN 'Saturday' THEN 6 WHEN 'Sunday' THEN 7 END, start_time`
		args = []any{sess.EntityID}
	} else {
		query = `SELECT id, day_of_week, start_time, end_time, COALESCE(teacher_id,''), COALESCE(room_id,''),
			COALESCE(subject,''), COALESCE(student_ids,''), COALESCE(effective_from,''), COALESCE(effective_until,'')
			FROM schedules WHERE deleted = 0
			ORDER BY CASE day_of_week WHEN 'Monday' THEN 1 WHEN 'Tuesday' THEN 2 WHEN 'Wednesday' THEN 3
			WHEN 'Thursday' THEN 4 WHEN 'Friday' THEN 5 WHEN 'Saturday' THEN 6 WHEN 'Sunday' THEN 7 END, start_time`
	}

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	defer rows.Close()

	type classInfo struct {
		ID             string   `json:"id"`
		DayOfWeek      string   `json:"day_of_week"`
		StartTime      string   `json:"start_time"`
		EndTime        string   `json:"end_time"`
		TeacherID      string   `json:"teacher_id"`
		TeacherName    string   `json:"teacher_name"`
		RoomID         string   `json:"room_id"`
		RoomName       string   `json:"room_name"`
		Subject        string   `json:"subject"`
		StudentIDs     []string `json:"student_ids"`
		Students       []any    `json:"students"`
		EffectiveFrom  string   `json:"effective_from"`
		EffectiveUntil string   `json:"effective_until"`
		Capacity       int      `json:"capacity"`
		EnrolledCount  int      `json:"enrolled_count"`
		IsEnrolled     bool     `json:"is_enrolled"`
	}

	var classes []classInfo
	for rows.Next() {
		var c classInfo
		var studentIDsStr, teacherID string
		if err := rows.Scan(&c.ID, &c.DayOfWeek, &c.StartTime, &c.EndTime, &teacherID, &c.RoomID,
			&c.Subject, &studentIDsStr, &c.EffectiveFrom, &c.EffectiveUntil); err != nil {
			continue
		}
		c.TeacherID = teacherID
		c.StudentIDs = splitSemicolon(studentIDsStr)
		c.EnrolledCount = len(c.StudentIDs)

		// Resolve teacher name
		if teacherID != "" {
			var fn, ln string
			if err := a.DB.QueryRow("SELECT first_name, last_name FROM teachers WHERE id = ?", teacherID).Scan(&fn, &ln); err == nil {
				c.TeacherName = fn + " " + ln
			}
		}

		// Resolve room name and capacity
		if c.RoomID != "" {
			var rname string
			var cap sql.NullInt64
			if err := a.DB.QueryRow("SELECT name, COALESCE(capacity, 0) FROM rooms WHERE id = ?", c.RoomID).Scan(&rname, &cap); err == nil {
				c.RoomName = rname
				c.Capacity = int(cap.Int64)
			}
		}

		// Resolve student names
		for _, sid := range c.StudentIDs {
			var fn, ln string
			if err := a.DB.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", sid).Scan(&fn, &ln); err == nil {
				c.Students = append(c.Students, map[string]string{"id": sid, "name": fn + " " + ln})
			}
		}
		if c.Students == nil {
			c.Students = []any{}
		}

		// Check enrollment for the relevant student
		if checkStudentID != "" {
			for _, sid := range c.StudentIDs {
				if sid == checkStudentID {
					c.IsEnrolled = true
					break
				}
			}
		}

		classes = append(classes, c)
	}
	if classes == nil {
		classes = []classInfo{}
	}
	writeJSON(w, http.StatusOK, classes)
}

// HandleDashboardEnroll enrolls a student in a class.
func (a *App) HandleDashboardEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	var req struct {
		ScheduleID string `json:"schedule_id"`
		StudentID  string `json:"student_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ScheduleID == "" || req.StudentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "schedule_id and student_id required"})
		return
	}

	// Auth: student can only enroll self; parent can enroll their children
	if err := a.verifyStudentAccess(sess, req.StudentID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}

	// Get current schedule
	var studentIDsStr, roomID string
	var deleted int
	err := a.DB.QueryRow("SELECT COALESCE(student_ids,''), COALESCE(room_id,''), deleted FROM schedules WHERE id = ?", req.ScheduleID).
		Scan(&studentIDsStr, &roomID, &deleted)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Schedule not found"})
		return
	}
	if deleted != 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Schedule is deleted"})
		return
	}

	ids := splitSemicolon(studentIDsStr)

	// Check duplicate
	for _, sid := range ids {
		if sid == req.StudentID {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "Already enrolled"})
			return
		}
	}

	// Check capacity
	if roomID != "" {
		var cap int
		if err := a.DB.QueryRow("SELECT COALESCE(capacity, 0) FROM rooms WHERE id = ?", roomID).Scan(&cap); err == nil && cap > 0 {
			if len(ids) >= cap {
				writeJSON(w, http.StatusConflict, map[string]any{"error": "Class is full"})
				return
			}
		}
	}

	// Add student
	ids = append(ids, req.StudentID)
	newVal := strings.Join(ids, ";")
	if _, err := a.DB.Exec("UPDATE schedules SET student_ids = ? WHERE id = ?", newVal, req.ScheduleID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Failed to enroll"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleDashboardUnenroll removes a student from a class.
func (a *App) HandleDashboardUnenroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	var req struct {
		ScheduleID string `json:"schedule_id"`
		StudentID  string `json:"student_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ScheduleID == "" || req.StudentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "schedule_id and student_id required"})
		return
	}

	// Auth check
	if err := a.verifyStudentAccess(sess, req.StudentID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
		return
	}

	// Get current schedule
	var studentIDsStr string
	err := a.DB.QueryRow("SELECT COALESCE(student_ids,'') FROM schedules WHERE id = ? AND deleted = 0", req.ScheduleID).Scan(&studentIDsStr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Schedule not found"})
		return
	}

	ids := splitSemicolon(studentIDsStr)
	var newIDs []string
	found := false
	for _, sid := range ids {
		if sid == req.StudentID {
			found = true
		} else {
			newIDs = append(newIDs, sid)
		}
	}
	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Not enrolled"})
		return
	}

	newVal := strings.Join(newIDs, ";")
	if _, err := a.DB.Exec("UPDATE schedules SET student_ids = ? WHERE id = ?", newVal, req.ScheduleID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Failed to unenroll"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleDashboardScheduleSave creates or updates a schedule for the logged-in teacher.
func (a *App) HandleDashboardScheduleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}
	if sess.UserType != "teacher" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "Teachers only"})
		return
	}

	var req struct {
		ID             string `json:"id"`
		DayOfWeek      string `json:"day_of_week"`
		StartTime      string `json:"start_time"`
		EndTime        string `json:"end_time"`
		RoomID         string `json:"room_id"`
		Subject        string `json:"subject"`
		StudentIDs     string `json:"student_ids"`
		EffectiveFrom  string `json:"effective_from"`
		EffectiveUntil string `json:"effective_until"`
		Type           string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	if req.DayOfWeek == "" || req.StartTime == "" || req.EndTime == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "day_of_week, start_time, and end_time are required"})
		return
	}

	teacherID := sess.EntityID

	if req.ID == "" {
		// Auto-generate ID
		var maxID string
		_ = a.DB.QueryRow("SELECT id FROM schedules WHERE id LIKE 'SCH%' ORDER BY CAST(SUBSTR(id, 4) AS INTEGER) DESC LIMIT 1").Scan(&maxID)
		next := 1
		if maxID != "" {
			fmt.Sscanf(maxID[3:], "%d", &next)
			next++
		}
		req.ID = fmt.Sprintf("SCH%03d", next)
	} else {
		// Verify the teacher owns this schedule
		var ownerID string
		err := a.DB.QueryRow("SELECT COALESCE(teacher_id,'') FROM schedules WHERE id = ? AND deleted = 0", req.ID).Scan(&ownerID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "Schedule not found"})
			return
		}
		if ownerID != teacherID {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "Not your schedule"})
			return
		}
	}

	schedType := req.Type
	if schedType == "" {
		schedType = "class"
	}
	_, err := a.DB.Exec(
		`INSERT OR REPLACE INTO schedules (id, day_of_week, start_time, end_time, teacher_id, room_id, subject, student_ids, effective_from, effective_until, type, deleted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		req.ID, req.DayOfWeek, req.StartTime, req.EndTime, teacherID, req.RoomID,
		req.Subject, req.StudentIDs, req.EffectiveFrom, req.EffectiveUntil, schedType,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Failed to save"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}

// HandleDashboardScheduleDelete soft-deletes a schedule owned by the logged-in teacher.
func (a *App) HandleDashboardScheduleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}
	if sess.UserType != "teacher" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "Teachers only"})
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
		return
	}

	// Verify ownership
	var ownerID string
	err := a.DB.QueryRow("SELECT COALESCE(teacher_id,'') FROM schedules WHERE id = ? AND deleted = 0", req.ID).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Schedule not found"})
		return
	}
	if ownerID != sess.EntityID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "Not your schedule"})
		return
	}

	_, err = a.DB.Exec("UPDATE schedules SET deleted = 1, deleted_at = datetime('now','localtime'), deleted_by = ? WHERE id = ?",
		sess.EntityID, req.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Failed to delete"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleDashboardRooms returns all rooms with capacity.
func (a *App) HandleDashboardRooms(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	rows, err := a.DB.Query("SELECT id, COALESCE(name,''), COALESCE(capacity, 0) FROM rooms WHERE deleted = 0 ORDER BY name")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	defer rows.Close()

	type roomInfo struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Capacity int    `json:"capacity"`
	}
	var rooms []roomInfo
	for rows.Next() {
		var r roomInfo
		if err := rows.Scan(&r.ID, &r.Name, &r.Capacity); err != nil {
			continue
		}
		rooms = append(rooms, r)
	}
	if rooms == nil {
		rooms = []roomInfo{}
	}
	writeJSON(w, http.StatusOK, rooms)
}

// verifyStudentAccess checks that the session user is allowed to act on behalf of the given student.
func (a *App) verifyStudentAccess(sess *auth.Session, studentID string) error {
	switch sess.UserType {
	case "student":
		if sess.EntityID != studentID {
			return fmt.Errorf("Cannot act on behalf of another student")
		}
	case "parent":
		childIDs, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
		for _, cid := range childIDs {
			if cid == studentID {
				return nil
			}
		}
		return fmt.Errorf("Not your child")
	case "teacher":
		// Teachers can also enroll/unenroll students in their classes
		return nil
	default:
		return fmt.Errorf("Unauthorized")
	}
	return nil
}
