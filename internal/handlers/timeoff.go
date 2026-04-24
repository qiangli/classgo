package handlers

import (
	"encoding/json"
	"net/http"

	"classgo/internal/database"
	"classgo/internal/models"
)

// HandleTimeOffList returns time-off records filtered by query params.
// GET /api/v1/timeoff?user_id=&user_type=&from=&to=
func (a *App) HandleTimeOffList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.URL.Query().Get("user_id")
	userType := r.URL.Query().Get("user_type")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	records, err := database.ListTimeOff(a.DB, userID, userType, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if records == nil {
		records = []models.TimeOff{}
	}
	writeJSON(w, http.StatusOK, records)
}

// HandleTimeOffSave creates or updates a time-off record.
// POST /api/v1/timeoff/save
func (a *App) HandleTimeOffSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID           int     `json:"id"`
		UserID       string  `json:"user_id"`
		UserType     string  `json:"user_type"`
		Date         string  `json:"date"`
		Type         string  `json:"type"`
		ScheduleType string  `json:"schedule_type"`
		Hours        float64 `json:"hours"`
		Notes        string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	if req.UserID == "" || req.Date == "" || req.Type == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user_id, date, and type are required"})
		return
	}
	validTypes := map[string]bool{"holiday": true, "sick": true, "personal": true}
	if !validTypes[req.Type] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "type must be holiday, sick, or personal"})
		return
	}

	createdBy := "admin"
	if sess := a.GetSession(r); sess != nil {
		if sess.EntityID != "" {
			createdBy = sess.EntityID
		} else {
			createdBy = sess.Username
		}
	}

	if req.ID > 0 {
		if err := database.UpdateTimeOff(a.DB, req.ID, req.Type, req.ScheduleType, req.Hours, req.Notes); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
	} else {
		id, err := database.CreateTimeOff(a.DB, req.UserID, req.UserType, req.Date, req.Type, req.ScheduleType, req.Hours, req.Notes, createdBy)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	}
}

// HandleTimeOffDelete deletes a time-off record.
// POST /api/v1/timeoff/delete
func (a *App) HandleTimeOffDelete(w http.ResponseWriter, r *http.Request) {
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
	if err := database.DeleteTimeOff(a.DB, req.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleDashboardMyTimeOff returns time-off records for the logged-in user.
// GET /api/dashboard/my-timeoff?from=&to=
func (a *App) HandleDashboardMyTimeOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	records, err := database.ListTimeOff(a.DB, sess.EntityID, sess.UserType, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if records == nil {
		records = []models.TimeOff{}
	}
	writeJSON(w, http.StatusOK, records)
}
