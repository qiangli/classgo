package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/models"
)

// ==================== ADMIN PROFILE (existing) ====================

// HandleProfilePage serves the admin student profile form page.
func (a *App) HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("id")
	if studentID == "" {
		http.Error(w, "Student ID required", http.StatusBadRequest)
		return
	}
	data := struct {
		AppName   string
		StudentID string
	}{a.AppName, studentID}
	a.Tmpl.ExecuteTemplate(w, "profile.html", data)
}

// HandleStudentProfile returns or saves student + parent profile data (admin-only).
func (a *App) HandleStudentProfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getStudentProfile(w, r)
	case http.MethodPost:
		a.saveStudentProfile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) getStudentProfile(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("id")
	if studentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Student ID required"})
		return
	}
	student, parent, err := a.fetchStudentProfile(studentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Student not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "student": student, "parent": parent})
}

func (a *App) saveStudentProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Student  map[string]any `json:"student"`
		Parent   map[string]any `json:"parent"`
		Finalize bool           `json:"finalize"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.Student != nil {
		id := getString(req.Student, "id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Student ID required"})
			return
		}
		status := "draft"
		if req.Finalize {
			status = "final"
		}
		if err := a.updateStudentFields(id, req.Student, status); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to save student"})
			return
		}
	}
	if req.Parent != nil {
		if err := a.updateParentFields(req.Parent); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to save parent"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ==================== USER PROFILE (new) ====================

// HandleUserProfilePage serves the standalone profile page.
func (a *App) HandleUserProfilePage(w http.ResponseWriter, r *http.Request) {
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
	a.Tmpl.ExecuteTemplate(w, "profile_standalone.html", data)
}

// HandleUserProfile handles GET/POST for user-facing profile (RequireAuth).
func (a *App) HandleUserProfile(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Not authenticated"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.getUserProfile(w, r, sess)
	case http.MethodPost:
		a.saveUserProfile(w, r, sess)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) getUserProfile(w http.ResponseWriter, r *http.Request, sess *auth.Session) {
	// Teachers have their own profile — not a student profile
	if sess.UserType == "teacher" || sess.UserType == "admin" {
		a.getTeacherProfile(w, sess)
		return
	}

	studentID := a.resolveStudentID(sess, r)
	if studentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "No student profile available"})
		return
	}
	if !a.canAccessStudentProfile(sess, studentID) {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "Access denied"})
		return
	}

	student, parent, err := a.fetchStudentProfile(studentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Student not found"})
		return
	}

	// Get global tracker items and latest values
	trackerItems, _ := database.GetGlobalTrackerItems(a.DB)
	trackerValues, _ := database.GetLatestTrackerValues(a.DB, studentID)

	// Determine if profile is empty
	isEmptyProfile := student["dob"] == "" && student["school"] == "" && student["grade"] == ""

	// Get children list for parents
	var children []map[string]string
	if sess.UserType == "parent" {
		childIDs, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
		for _, cid := range childIDs {
			var fn, ln string
			a.DB.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", cid).Scan(&fn, &ln)
			children = append(children, map[string]string{"id": cid, "name": fn + " " + ln})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"student":          student,
		"parent":           parent,
		"tracker_items":    trackerItems,
		"tracker_values":   trackerValues,
		"is_empty_profile": isEmptyProfile,
		"children":         children,
	})
}

func (a *App) getTeacherProfile(w http.ResponseWriter, sess *auth.Session) {
	var id, fn, ln, email, phone, address, subjects string
	err := a.DB.QueryRow(`SELECT id, first_name, last_name, COALESCE(email,''), COALESCE(phone,''),
		COALESCE(address,''), COALESCE(subjects,'') FROM teachers WHERE id = ?`, sess.EntityID).
		Scan(&id, &fn, &ln, &email, &phone, &address, &subjects)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Teacher not found"})
		return
	}
	teacher := map[string]any{
		"id": id, "first_name": fn, "last_name": ln,
		"email": email, "phone": phone, "address": address, "subjects": subjects,
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"teacher": teacher,
		"student": teacher, // frontend reads student fields for display
	})
}

func (a *App) saveTeacherProfile(w http.ResponseWriter, sess *auth.Session, data map[string]any) {
	_, err := a.DB.Exec(`UPDATE teachers SET
		first_name = ?, last_name = ?, email = ?, phone = ?, address = ?
		WHERE id = ?`,
		getString(data, "first_name"), getString(data, "last_name"),
		getString(data, "email"), getString(data, "phone"),
		getString(data, "address"),
		sess.EntityID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to save profile"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) saveUserProfile(w http.ResponseWriter, r *http.Request, sess *auth.Session) {
	var req struct {
		Student       map[string]any `json:"student"`
		Parent        map[string]any `json:"parent"`
		TrackerValues map[string]any `json:"tracker_values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	// Teachers save to the teachers table
	if sess.UserType == "teacher" || sess.UserType == "admin" {
		if req.Student != nil {
			a.saveTeacherProfile(w, sess, req.Student)
		} else {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		}
		return
	}

	studentID := ""
	if req.Student != nil {
		studentID = getString(req.Student, "id")
	}
	if studentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Student ID required"})
		return
	}
	if !a.canAccessStudentProfile(sess, studentID) {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "Access denied"})
		return
	}

	// Save student fields with draft status
	if req.Student != nil {
		if err := a.updateStudentFields(studentID, req.Student, "draft"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to save student"})
			return
		}
	}
	if req.Parent != nil {
		if err := a.updateParentFields(req.Parent); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to save parent"})
			return
		}
	}

	// Save tracker values
	if len(req.TrackerValues) > 0 {
		values := make(map[int]string)
		for k, v := range req.TrackerValues {
			id, err := strconv.Atoi(k)
			if err != nil {
				continue
			}
			if s, ok := v.(string); ok && s != "" {
				values[id] = s
			}
		}
		studentName := getString(req.Student, "first_name") + " " + getString(req.Student, "last_name")
		if err := database.SaveProfileTrackerValues(a.DB, studentID, studentName, values); err != nil {
			log.Printf("Failed to save tracker values: %v", err)
		}
	}

	// Auto-assign tasks for profile gaps
	grade := getString(req.Student, "grade")
	if err := database.AutoAssignProfileTasks(a.DB, studentID, grade); err != nil {
		log.Printf("Failed to auto-assign profile tasks: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ==================== SHARED HELPERS ====================

// fetchStudentProfile returns student and parent data as maps.
func (a *App) fetchStudentProfile(studentID string) (map[string]any, map[string]any, error) {
	row := a.DB.QueryRow(`SELECT id, first_name, last_name, COALESCE(grade,''), COALESCE(school,''), COALESCE(parent_id,''),
		COALESCE(email,''), COALESCE(phone,''), COALESCE(address,''), COALESCE(notes,''),
		COALESCE(dob,''), COALESCE(birthplace,''), COALESCE(years_in_us,''), COALESCE(first_language,''),
		COALESCE(previous_schools,''), COALESCE(courses_outside,''), COALESCE(profile_status,''), active
		FROM students WHERE id = ?`, studentID)

	var id, fn, ln, grade, school, parentID, email, phone, address, notes string
	var dob, birthplace, yearsInUS, firstLang, prevSchools, coursesOutside, profileStatus string
	var active int
	if err := row.Scan(&id, &fn, &ln, &grade, &school, &parentID,
		&email, &phone, &address, &notes,
		&dob, &birthplace, &yearsInUS, &firstLang,
		&prevSchools, &coursesOutside, &profileStatus, &active); err != nil {
		return nil, nil, err
	}

	student := map[string]any{
		"id": id, "first_name": fn, "last_name": ln, "grade": grade, "school": school,
		"parent_id": parentID, "email": email, "phone": phone, "address": address, "notes": notes,
		"dob": dob, "birthplace": birthplace, "years_in_us": yearsInUS, "first_language": firstLang,
		"previous_schools": prevSchools, "courses_outside": coursesOutside,
		"profile_status": profileStatus, "active": active,
	}

	var parent map[string]any
	if parentID != "" {
		var pid, pfn, pln, pemail, pphone, pemail2, pphone2, paddress string
		err := a.DB.QueryRow(`SELECT id, first_name, last_name, COALESCE(email,''), COALESCE(phone,''),
			COALESCE(email2,''), COALESCE(phone2,''), COALESCE(address,'')
			FROM parents WHERE id = ?`, parentID).Scan(
			&pid, &pfn, &pln, &pemail, &pphone, &pemail2, &pphone2, &paddress)
		if err == nil {
			parent = map[string]any{
				"id": pid, "first_name": pfn, "last_name": pln,
				"email": pemail, "phone": pphone, "email2": pemail2, "phone2": pphone2, "address": paddress,
			}
		}
	}
	return student, parent, nil
}

func (a *App) updateStudentFields(id string, data map[string]any, status string) error {
	_, err := a.DB.Exec(`UPDATE students SET
		first_name = ?, last_name = ?, grade = ?, school = ?, parent_id = ?,
		email = ?, phone = ?, address = ?, notes = ?,
		dob = ?, birthplace = ?, years_in_us = ?, first_language = ?,
		previous_schools = ?, courses_outside = ?, profile_status = ?
		WHERE id = ?`,
		getString(data, "first_name"), getString(data, "last_name"),
		getString(data, "grade"), getString(data, "school"),
		getString(data, "parent_id"),
		getString(data, "email"), getString(data, "phone"),
		getString(data, "address"), getString(data, "notes"),
		getString(data, "dob"), getString(data, "birthplace"),
		getString(data, "years_in_us"), getString(data, "first_language"),
		getString(data, "previous_schools"), getString(data, "courses_outside"),
		status, id,
	)
	return err
}

func (a *App) updateParentFields(data map[string]any) error {
	id := getString(data, "id")
	if id == "" {
		return nil
	}
	_, err := a.DB.Exec(`UPDATE parents SET
		first_name = ?, last_name = ?, email = ?, phone = ?,
		email2 = ?, phone2 = ?, address = ?
		WHERE id = ?`,
		getString(data, "first_name"), getString(data, "last_name"),
		getString(data, "email"), getString(data, "phone"),
		getString(data, "email2"), getString(data, "phone2"),
		getString(data, "address"),
		id,
	)
	return err
}

// resolveStudentID determines the student ID based on session context.
func (a *App) resolveStudentID(sess *auth.Session, r *http.Request) string {
	switch sess.UserType {
	case "student":
		return sess.EntityID
	case "parent":
		if sid := r.URL.Query().Get("student_id"); sid != "" {
			return sid
		}
		ids, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
		if len(ids) > 0 {
			return ids[0]
		}
	}
	return ""
}

// canAccessStudentProfile checks if the session user can access the given student's profile.
func (a *App) canAccessStudentProfile(sess *auth.Session, studentID string) bool {
	switch sess.UserType {
	case "student":
		return sess.EntityID == studentID
	case "parent":
		ids, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
		for _, id := range ids {
			if id == studentID {
				return true
			}
		}
	}
	return false
}
