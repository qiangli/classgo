package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/models"
)

func (a *App) HandleSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		PIN         string `json:"pin"`
		DeviceType  string `json:"device_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.StudentName == "" || req.PIN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Name and PIN are required"})
		return
	}

	if req.DeviceType != "mobile" && req.DeviceType != "kiosk" {
		req.DeviceType = "mobile"
	}

	pin := a.EnsureDailyPIN()
	if req.PIN != pin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid PIN"})
		return
	}

	var existingID int
	err := a.DB.QueryRow(
		"SELECT id FROM attendance WHERE student_name = ? AND date(sign_in_time) = date('now','localtime') AND sign_out_time IS NULL LIMIT 1",
		req.StudentName,
	).Scan(&existingID)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Already signed in today!"})
		return
	}

	_, err = a.DB.Exec(
		"INSERT INTO attendance (student_name, device_type) VALUES (?, ?)",
		req.StudentName, req.DeviceType,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to record attendance"})
		log.Printf("Insert error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Welcome, %s!", req.StudentName)})
}

func (a *App) HandleStatus(w http.ResponseWriter, r *http.Request) {
	studentName := r.URL.Query().Get("student_name")
	if studentName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"signed_in": false, "error": "student_name required"})
		return
	}

	var id int
	var signOutTime sql.NullString
	err := a.DB.QueryRow(
		"SELECT id, sign_out_time FROM attendance WHERE student_name = ? AND date(sign_in_time) = date('now','localtime') ORDER BY sign_in_time DESC LIMIT 1",
		studentName,
	).Scan(&id, &signOutTime)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{"signed_in": false})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"signed_in": false, "error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"signed_in":  true,
		"signed_out": signOutTime.Valid,
	})
}

func (a *App) HandleSignOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		PIN         string `json:"pin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.StudentName == "" || req.PIN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Name and PIN are required"})
		return
	}

	pin := a.EnsureDailyPIN()
	if req.PIN != pin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid PIN"})
		return
	}

	result, err := a.DB.Exec(
		"UPDATE attendance SET sign_out_time = datetime('now','localtime') WHERE student_name = ? AND date(sign_in_time) = date('now','localtime') AND sign_out_time IS NULL",
		req.StudentName,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "No active sign-in found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Goodbye, %s!", req.StudentName)})
}

func (a *App) HandleAttendees(w http.ResponseWriter, r *http.Request) {
	attendees, err := database.TodayAttendees(a.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if attendees == nil {
		attendees = []models.Attendance{}
	}
	writeJSON(w, http.StatusOK, attendees)
}

func (a *App) HandleExport(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	var rows *sql.Rows
	var err error

	if from != "" && to != "" {
		rows, err = a.DB.Query(
			"SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance WHERE date(sign_in_time) BETWEEN ? AND ? ORDER BY sign_in_time DESC",
			from, to,
		)
	} else {
		rows, err = a.DB.Query(
			"SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance ORDER BY sign_in_time DESC",
		)
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("classgo-export-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Student Name", "Device Type", "Sign In", "Sign Out", "Duration"})

	for rows.Next() {
		var id int
		var studentName, deviceType, signIn string
		var signOut sql.NullString
		if err := rows.Scan(&id, &studentName, &deviceType, &signIn, &signOut); err != nil {
			continue
		}
		inTime, _ := models.ParseTimestamp(signIn)
		signInFmt := inTime.Format("2006-01-02 3:04 PM")
		signOutFmt := ""
		durationStr := ""
		if signOut.Valid {
			outTime, _ := models.ParseTimestamp(signOut.String)
			signOutFmt = outTime.Format("2006-01-02 3:04 PM")
			durationStr = models.FormatDuration(outTime.Sub(inTime))
		}
		writer.Write([]string{fmt.Sprintf("%d", id), studentName, deviceType, signInFmt, signOutFmt, durationStr})
	}
	writer.Flush()
}

func (a *App) HandleExportXLSX(w http.ResponseWriter, r *http.Request) {
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Printf("Export XLSX read error: %v", err)
		return
	}

	f, err := datastore.ExportXLSX(a.DB, data)
	if err != nil {
		http.Error(w, "Export error", http.StatusInternalServerError)
		log.Printf("Export XLSX error: %v", err)
		return
	}
	defer f.Close()

	filename := fmt.Sprintf("classgo-export-%s.xlsx", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	f.Write(w)
}
