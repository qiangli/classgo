package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/models"
	"classgo/internal/scheduling"
)

func (a *App) HandleStudentSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	students, err := database.SearchStudents(a.DB, q, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if students == nil {
		students = []models.Student{}
	}
	writeJSON(w, http.StatusOK, students)
}

func (a *App) HandleCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		StudentID   string `json:"student_id"`
		PIN         string `json:"pin"`
		DeviceType  string `json:"device_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	// If student_id provided, look up the name
	if req.StudentID != "" && req.StudentName == "" {
		req.StudentName = a.lookupStudentName(req.StudentID)
	}

	if req.StudentName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Name is required"})
		return
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

	if req.DeviceType != "mobile" && req.DeviceType != "kiosk" {
		req.DeviceType = "mobile"
	}

	var existingID int
	err := a.DB.QueryRow(
		"SELECT id FROM attendance WHERE student_name = ? AND date(check_in_time) = date('now','localtime') AND check_out_time IS NULL LIMIT 1",
		req.StudentName,
	).Scan(&existingID)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Already checked in today!"})
		return
	}

	result, err := a.DB.Exec(
		"INSERT INTO attendance (student_name, device_type) VALUES (?, ?)",
		req.StudentName, req.DeviceType,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to record attendance"})
		log.Printf("Insert error: %v", err)
		return
	}

	// Link attendance to student and schedule
	if attendanceID, err := result.LastInsertId(); err == nil {
		if req.StudentID != "" {
			a.linkAttendanceMetaByID(attendanceID, req.StudentID)
		} else {
			a.linkAttendanceMeta(attendanceID, req.StudentName)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Welcome, %s!", req.StudentName)})
}

func (a *App) HandleStatus(w http.ResponseWriter, r *http.Request) {
	studentName := r.URL.Query().Get("student_name")
	if studentName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"checked_in": false, "error": "student_name required"})
		return
	}

	var id int
	var checkOutTime sql.NullString
	err := a.DB.QueryRow(
		"SELECT id, check_out_time FROM attendance WHERE student_name = ? AND date(check_in_time) = date('now','localtime') ORDER BY check_in_time DESC LIMIT 1",
		studentName,
	).Scan(&id, &checkOutTime)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{"checked_in": false})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"checked_in": false, "error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"checked_in":  true,
		"checked_out": checkOutTime.Valid,
	})
}

func (a *App) HandleCheckOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		StudentID   string `json:"student_id"`
		PIN         string `json:"pin"`
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

	result, err := a.DB.Exec(
		"UPDATE attendance SET check_out_time = datetime('now','localtime') WHERE student_name = ? AND date(check_in_time) = date('now','localtime') AND check_out_time IS NULL",
		req.StudentName,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "No active check-in found"})
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
			"SELECT id, student_name, device_type, check_in_time, check_out_time FROM attendance WHERE date(check_in_time) BETWEEN ? AND ? ORDER BY check_in_time DESC",
			from, to,
		)
	} else {
		rows, err = a.DB.Query(
			"SELECT id, student_name, device_type, check_in_time, check_out_time FROM attendance ORDER BY check_in_time DESC",
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
	writer.Write([]string{"ID", "Student Name", "Device Type", "Check In", "Check Out", "Duration"})

	for rows.Next() {
		var id int
		var studentName, deviceType, checkIn string
		var checkOut sql.NullString
		if err := rows.Scan(&id, &studentName, &deviceType, &checkIn, &checkOut); err != nil {
			continue
		}
		inTime, _ := models.ParseTimestamp(checkIn)
		checkInFmt := inTime.Format("2006-01-02 3:04 PM")
		checkOutFmt := ""
		durationStr := ""
		if checkOut.Valid {
			outTime, _ := models.ParseTimestamp(checkOut.String)
			checkOutFmt = outTime.Format("2006-01-02 3:04 PM")
			durationStr = models.FormatDuration(outTime.Sub(inTime))
		}
		writer.Write([]string{fmt.Sprintf("%d", id), studentName, deviceType, checkInFmt, checkOutFmt, durationStr})
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

// lookupStudentName returns "FirstName LastName" for a given student ID.
func (a *App) lookupStudentName(studentID string) string {
	var firstName, lastName string
	err := a.DB.QueryRow(
		"SELECT first_name, last_name FROM students WHERE id = ? AND active = 1",
		studentID,
	).Scan(&firstName, &lastName)
	if err != nil {
		return ""
	}
	return firstName + " " + lastName
}

// linkAttendanceMetaByID links an attendance record directly using a known student ID.
func (a *App) linkAttendanceMetaByID(attendanceID int64, studentID string) {
	scheduleID := a.findCurrentSchedule(studentID)
	_, err := a.DB.Exec(
		"INSERT OR REPLACE INTO attendance_meta (attendance_id, student_id, schedule_id) VALUES (?, ?, ?)",
		attendanceID, studentID, scheduleID,
	)
	if err != nil {
		log.Printf("linkAttendanceMetaByID error: %v", err)
	}
}

// linkAttendanceMeta tries to match a student name to structured data and
// link the attendance record to the student and their current scheduled session.
func (a *App) linkAttendanceMeta(attendanceID int64, studentName string) {
	studentID := a.findStudentID(studentName)
	if studentID == "" {
		return
	}

	scheduleID := a.findCurrentSchedule(studentID)

	_, err := a.DB.Exec(
		"INSERT OR REPLACE INTO attendance_meta (attendance_id, student_id, schedule_id) VALUES (?, ?, ?)",
		attendanceID, studentID, scheduleID,
	)
	if err != nil {
		log.Printf("linkAttendanceMeta error: %v", err)
	}
}

func (a *App) findStudentID(name string) string {
	name = strings.TrimSpace(name)
	nameLower := strings.ToLower(name)

	var id string
	err := a.DB.QueryRow(
		"SELECT id FROM students WHERE LOWER(first_name || ' ' || last_name) = ? AND active = 1 LIMIT 1",
		nameLower,
	).Scan(&id)
	if err == nil {
		return id
	}

	err = a.DB.QueryRow(
		"SELECT id FROM students WHERE LOWER(first_name) = ? AND active = 1 LIMIT 1",
		nameLower,
	).Scan(&id)
	if err == nil {
		return id
	}

	return ""
}

func (a *App) findCurrentSchedule(studentID string) string {
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sessions := scheduling.MaterializeSessions(data.Schedules, today, today)
	currentTime := now.Format("15:04")

	for _, s := range sessions {
		inSession := false
		for _, sid := range s.StudentIDs {
			if sid == studentID {
				inSession = true
				break
			}
		}
		if !inSession {
			continue
		}

		graceStart := subtractMinutes(s.StartTime, 30)
		if currentTime >= graceStart && currentTime <= s.EndTime {
			return s.ScheduleID
		}
	}

	return ""
}

func subtractMinutes(timeStr string, minutes int) string {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return timeStr
	}
	t = t.Add(-time.Duration(minutes) * time.Minute)
	return t.Format("15:04")
}
