package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/models"
)

// HandleTrackerDue returns due tracker items for a student today.
func (a *App) HandleTrackerDue(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	studentName := r.URL.Query().Get("student_name")

	if studentID == "" && studentName != "" {
		studentID = a.findStudentID(studentName)
	}
	if studentID == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	today := time.Now().Format("2006-01-02")
	items, err := database.GetDueItems(a.DB, studentID, today)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if items == nil {
		items = []models.DueItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

// HandleTrackerRespond submits tracker responses and completes checkout atomically.
func (a *App) HandleTrackerRespond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		StudentID   string `json:"student_id"`
		PIN         string `json:"pin"`
		Responses   []struct {
			ItemType string `json:"item_type"`
			ItemID   int    `json:"item_id"`
			ItemName string `json:"item_name"`
			Status   string `json:"status"`
			Notes    string `json:"notes"`
		} `json:"responses"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.StudentID != "" && req.StudentName == "" {
		req.StudentName = a.lookupStudentName(req.StudentID)
	}
	if req.StudentName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Name is required"})
		return
	}
	if req.StudentID == "" {
		req.StudentID = a.findStudentID(req.StudentName)
	}

	if a.RequirePIN() {
		if req.PIN == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "PIN is required"})
			return
		}
		pin := a.EnsureDailyPIN()
		if req.PIN != pin {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid PIN"})
			return
		}
	}

	for _, resp := range req.Responses {
		if resp.Status != "done" && resp.Status != "not_done" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid status: " + resp.Status})
			return
		}
		if resp.ItemType != "global" && resp.ItemType != "adhoc" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid item type"})
			return
		}
	}

	var responses []models.TrackerResponse
	for _, resp := range req.Responses {
		responses = append(responses, models.TrackerResponse{
			ItemType: resp.ItemType,
			ItemID:   resp.ItemID,
			ItemName: resp.ItemName,
			Status:   resp.Status,
			Notes:    resp.Notes,
		})
	}

	rows, err := database.SaveTrackerResponses(a.DB, req.StudentID, req.StudentName, responses)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
		return
	}
	if rows == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "No active check-in found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Goodbye, " + req.StudentName + "!"})
}

// HandleTrackerItems handles CRUD for global tracker items (admin only).
func (a *App) HandleTrackerItems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeDeleted := r.URL.Query().Get("include_deleted") == "1"
		items, err := database.ListTrackerItems(a.DB, includeDeleted)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
			return
		}
		if items == nil {
			items = []models.TrackerItem{}
		}
		writeJSON(w, http.StatusOK, items)

	case http.MethodPost:
		var item models.TrackerItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
			return
		}
		if item.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Name is required"})
			return
		}
		if item.Priority == "" {
			item.Priority = "medium"
		}
		if item.Recurrence == "" {
			item.Recurrence = "daily"
		}
		if item.CreatedBy == "" {
			item.CreatedBy = "admin"
		}
		id, err := database.SaveTrackerItem(a.DB, item)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleTrackerItemDelete soft-deletes a global tracker item (admin only).
func (a *App) HandleTrackerItemDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	if err := database.DeleteTrackerItem(a.DB, req.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleTrackerResponses returns tracker responses for a student on a date (admin only).
func (a *App) HandleTrackerResponses(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	date := r.URL.Query().Get("date")
	if studentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "student_id is required"})
		return
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	responses, err := database.GetTrackerResponsesForDate(a.DB, studentID, date)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if responses == nil {
		responses = []models.TrackerResponse{}
	}
	writeJSON(w, http.StatusOK, responses)
}

// HandleStudentTrackerItems handles ad hoc tracker items for a specific student.
func (a *App) HandleStudentTrackerItems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		studentID := r.URL.Query().Get("student_id")
		if studentID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "student_id is required"})
			return
		}
		// Enforce access: users can only see items for students they have access to
		sess := a.GetSession(r)
		if sess != nil && sess.Role != "admin" {
			if !a.canAccessStudent(sess, studentID) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "Access denied"})
				return
			}
		}
		items, err := database.ListStudentTrackerItems(a.DB, studentID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
			return
		}
		if items == nil {
			items = []models.StudentTrackerItem{}
		}
		writeJSON(w, http.StatusOK, items)

	case http.MethodPost:
		var item models.StudentTrackerItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
			return
		}
		if item.StudentID == "" || item.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "student_id and name are required"})
			return
		}
		if item.Priority == "" {
			item.Priority = "medium"
		}
		if item.Recurrence == "" {
			item.Recurrence = "none"
		}
		// Enforce ownership: check session
		sess := a.GetSession(r)
		if sess != nil && sess.Role != "admin" {
			if item.ID > 0 {
				// Only owner can update
				existing, _ := database.ListStudentTrackerItems(a.DB, item.StudentID)
				for _, e := range existing {
					if e.ID == item.ID && e.CreatedBy != sess.EntityID {
						writeJSON(w, http.StatusForbidden, map[string]any{"error": "Only the owner can edit this item"})
						return
					}
				}
			} else {
				item.CreatedBy = sess.EntityID
				item.OwnerType = sess.UserType
			}
		}
		if item.CreatedBy == "" {
			item.CreatedBy = "admin"
		}
		if item.OwnerType == "" {
			item.OwnerType = "admin"
		}
		isNew := item.ID == 0
		id, err := database.SaveStudentTrackerItem(a.DB, item)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
			return
		}
		// Notify via Memos on new item creation
		if isNew && a.MemosSyncer != nil {
			studentName := a.lookupStudentName(item.StudentID)
			go a.MemosSyncer.Client().NotifyTaskAssigned(item.StudentID, studentName, item.Name, item.CreatedBy)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleStudentTrackerItemDelete soft-deletes a per-student ad hoc tracker item.
// Only the owner (created_by) or admin can delete.
func (a *App) HandleStudentTrackerItemDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	// Enforce ownership
	sess := a.GetSession(r)
	if sess != nil && sess.Role != "admin" {
		var createdBy string
		err := a.DB.QueryRow("SELECT COALESCE(created_by,'') FROM student_tracker_items WHERE id = ?", req.ID).Scan(&createdBy)
		if err == nil && createdBy != sess.EntityID {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "Only the owner can delete this item"})
			return
		}
	}
	if err := database.DeleteStudentTrackerItem(a.DB, req.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleTrackerComplete marks a one-time task as done/undone (student sign-off).
func (a *App) HandleTrackerComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID       int    `json:"id"`
		Complete bool   `json:"complete"`
		EntityID string `json:"entity_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	sess := a.GetSession(r)
	completedBy := req.EntityID
	if sess != nil {
		completedBy = sess.EntityID
	}
	var err error
	if req.Complete {
		err = database.CompleteStudentTrackerItem(a.DB, req.ID, completedBy)
	} else {
		err = database.UncompleteStudentTrackerItem(a.DB, req.ID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleTrackerBulkAssign creates the same item for multiple students in a class.
func (a *App) HandleTrackerBulkAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		StudentIDs []string `json:"student_ids"`
		ScheduleID string   `json:"schedule_id"`
		Name       string   `json:"name"`
		Notes      string   `json:"notes"`
		StartDate  string   `json:"start_date"`
		DueDate    string   `json:"due_date"`
		Priority   string   `json:"priority"`
		Recurrence string   `json:"recurrence"`
		Category   string   `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Name is required"})
		return
	}

	// If schedule_id provided, get student IDs from that schedule
	studentIDs := req.StudentIDs
	if req.ScheduleID != "" && len(studentIDs) == 0 {
		var sids string
		err := a.DB.QueryRow("SELECT COALESCE(student_ids,'') FROM schedules WHERE id = ?", req.ScheduleID).Scan(&sids)
		if err == nil {
			for _, id := range splitSemicolon(sids) {
				studentIDs = append(studentIDs, id)
			}
		}
	}
	if len(studentIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "No students specified"})
		return
	}

	sess := a.GetSession(r)
	createdBy := "admin"
	ownerType := "admin"
	if sess != nil {
		createdBy = sess.EntityID
		ownerType = sess.UserType
		if sess.Role == "admin" {
			ownerType = "admin"
		}
	}

	item := models.StudentTrackerItem{
		Name:       req.Name,
		Notes:      req.Notes,
		StartDate:  req.StartDate,
		DueDate:    req.DueDate,
		Priority:   req.Priority,
		Recurrence: req.Recurrence,
		Category:   req.Category,
		CreatedBy:  createdBy,
		OwnerType:  ownerType,
		Active:     true,
	}
	if item.Priority == "" {
		item.Priority = "medium"
	}
	if item.Recurrence == "" {
		item.Recurrence = "none"
	}

	if err := database.BulkCreateStudentItems(a.DB, studentIDs, item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	// Notify via Memos for each student
	if a.MemosSyncer != nil {
		go func() {
			for _, sid := range studentIDs {
				name := a.lookupStudentName(sid)
				a.MemosSyncer.Client().NotifyTaskAssigned(sid, name, req.Name, createdBy)
			}
		}()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(studentIDs)})
}

// HandleTrackerProgress returns completion stats for students.
func (a *App) HandleTrackerProgress(w http.ResponseWriter, r *http.Request) {
	studentID := r.URL.Query().Get("student_id")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	var studentIDs []string
	if studentID != "" {
		studentIDs = []string{studentID}
	} else {
		// Get student IDs based on session
		sess := a.GetSession(r)
		if sess != nil {
			switch sess.UserType {
			case "teacher":
				ids, _ := database.GetTeacherStudentIDs(a.DB, sess.EntityID)
				studentIDs = ids
			case "parent":
				ids, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
				studentIDs = ids
			case "student":
				studentIDs = []string{sess.EntityID}
			}
		}
	}

	if len(studentIDs) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	stats, err := database.GetProgressStats(a.DB, studentIDs, startDate, endDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if stats == nil {
		stats = []models.ProgressStats{}
	}
	writeJSON(w, http.StatusOK, stats)
}

// canAccessStudent checks if the session user has access to a student's data.
// Students can access their own. Parents can access their children. Teachers can access their scheduled students.
func (a *App) canAccessStudent(sess *auth.Session, studentID string) bool {
	if sess == nil || sess.Role == "admin" {
		return true
	}
	switch sess.UserType {
	case "student":
		return strings.EqualFold(sess.EntityID, studentID)
	case "parent":
		ids, _ := database.GetParentStudentIDs(a.DB, sess.EntityID)
		for _, id := range ids {
			if strings.EqualFold(id, studentID) {
				return true
			}
		}
		return false
	case "teacher":
		ids, _ := database.GetTeacherStudentIDs(a.DB, sess.EntityID)
		for _, id := range ids {
			if strings.EqualFold(id, studentID) {
				return true
			}
		}
		return false
	}
	return false
}

func splitSemicolon(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	sep := ";"
	if !strings.Contains(s, ";") {
		sep = ","
	}
	var result []string
	for _, p := range strings.Split(s, sep) {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
