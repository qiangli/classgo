package handlers

import (
	"database/sql"
	"net/http"
	"time"

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

	items, err := database.ListStudentTrackerItemsByCreator(a.DB, sess.EntityID)
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
