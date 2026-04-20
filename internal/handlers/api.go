package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/memos"
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
		Fingerprint string `json:"fingerprint"`
		DeviceID    string `json:"device_id"`
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

	// Reject check-in if student is not in the system
	if req.StudentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Student not found. Please check the name and try again."})
		return
	}

	// Validate PIN based on mode
	needsSetup, pinErr := a.ValidatePIN(req.StudentID, req.PIN)
	if needsSetup {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "needs_pin_setup": true, "student_id": req.StudentID})
		return
	}
	if pinErr != "" {
		if strings.Contains(pinErr, "required") {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "needs_pin": true, "student_id": req.StudentID, "error": pinErr})
		} else {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": pinErr})
		}
		return
	}

	if req.DeviceType != "mobile" && req.DeviceType != "kiosk" {
		req.DeviceType = "mobile"
	}

	// Rate limit check
	clientIP := ClientIP(r)
	if a.RateLimiter != nil {
		deviceKey := DeviceKey(clientIP, req.Fingerprint, req.DeviceID)
		if msg := a.RateLimiter.Check(deviceKey, req.StudentName, req.DeviceType); msg != "" {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": msg})
			return
		}
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

	var attendanceID int64
	if id, err := result.LastInsertId(); err == nil {
		attendanceID = id
		if req.StudentID != "" {
			a.linkAttendanceMetaByID(id, req.StudentID)
		} else {
			a.linkAttendanceMeta(id, req.StudentName)
		}
	}

	// Audit log + flag detection
	auditID, _ := database.InsertAudit(a.DB, models.CheckinAudit{
		AttendanceID: int(attendanceID),
		StudentName:  req.StudentName,
		StudentID:    req.StudentID,
		DeviceType:   req.DeviceType,
		ClientIP:     clientIP,
		Fingerprint:  req.Fingerprint,
		DeviceID:     req.DeviceID,
		Action:       "checkin",
	})
	database.FlagSuspiciousCheckins(a.DB, auditID, clientIP, req.Fingerprint, req.DeviceID, req.StudentName)

	// Record for rate limiter
	if a.RateLimiter != nil {
		deviceKey := DeviceKey(clientIP, req.Fingerprint, req.DeviceID)
		a.RateLimiter.Record(deviceKey, req.StudentName)
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
		Fingerprint string `json:"fingerprint"`
		DeviceID    string `json:"device_id"`
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

	// Validate PIN
	_, pinErr := a.ValidatePIN(req.StudentID, req.PIN)
	if pinErr != "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": pinErr})
		return
	}

	// Block checkout if student has pending signoff tasks
	if req.StudentID != "" {
		pending, _ := database.PendingSignoffItems(a.DB, req.StudentID)
		if len(pending) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":            false,
				"pending_tasks": true,
				"items":         pending,
				"error":         "Please complete required tasks before checking out",
			})
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

	// Audit log
	clientIP := ClientIP(r)
	var attendanceID int
	a.DB.QueryRow(
		"SELECT id FROM attendance WHERE student_name = ? AND date(check_in_time) = date('now','localtime') ORDER BY check_in_time DESC LIMIT 1",
		req.StudentName,
	).Scan(&attendanceID)

	database.InsertAudit(a.DB, models.CheckinAudit{
		AttendanceID: attendanceID,
		StudentName:  req.StudentName,
		StudentID:    req.StudentID,
		DeviceType:   "mobile",
		ClientIP:     clientIP,
		Fingerprint:  req.Fingerprint,
		DeviceID:     req.DeviceID,
		Action:       "checkout",
	})

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Goodbye, %s!", req.StudentName)})
}

func (a *App) HandleAttendees(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	studentID := r.URL.Query().Get("student_id")
	teacherID := r.URL.Query().Get("teacher_id")
	parentID := r.URL.Query().Get("parent_id")

	var attendees []models.Attendance
	var err error
	hasFilter := from != "" || to != "" || studentID != "" || teacherID != "" || parentID != ""
	if hasFilter {
		attendees, err = database.AttendeesByDateRange(a.DB, from, to, studentID, teacherID, parentID)
	} else {
		attendees, err = database.TodayAttendees(a.DB)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if attendees == nil {
		attendees = []models.Attendance{}
	}
	writeJSON(w, http.StatusOK, attendees)
}

// HandleAttendanceMetrics returns summary statistics for a date range.
func (a *App) HandleAttendanceMetrics(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	metrics, err := database.GetAttendanceMetrics(a.DB, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
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

func (a *App) HandleDirectoryAPI(w http.ResponseWriter, r *http.Request) {
	includeDeleted := r.URL.Query().Get("include_deleted") == "1"
	var data *datastore.EntityData
	var err error
	if includeDeleted {
		data, err = datastore.ReadFromDBAll(a.DB)
	} else {
		data, err = datastore.ReadFromDB(a.DB)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"students":  data.Students,
		"parents":   data.Parents,
		"teachers":  data.Teachers,
		"rooms":     data.Rooms,
		"schedules": data.Schedules,
	})
}

// HandleDataCRUD handles create, update, and soft-delete operations for all entity types.
func (a *App) HandleDataCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string         `json:"action"` // "save" or "delete"
		Type   string         `json:"type"`   // "students", "parents", etc.
		ID     string         `json:"id"`     // for delete
		Data   map[string]any `json:"data"`   // for save
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	switch req.Action {
	case "delete":
		if req.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "ID is required"})
			return
		}
		tables := map[string]bool{"students": true, "parents": true, "teachers": true, "rooms": true, "schedules": true}
		if !tables[req.Type] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Unknown type"})
			return
		}
		_, err := a.DB.Exec("UPDATE "+req.Type+" SET deleted = 1 WHERE id = ?", req.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case "save":
		if req.Data == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Data is required"})
			return
		}
		if err := a.saveEntity(req.Type, req.Data); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Unknown action"})
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getBool(m map[string]any, key string, defaultVal bool) bool {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true" || b == "yes" || b == "1" || b == "Yes"
	case float64:
		return b != 0
	}
	return defaultVal
}

func getInt(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func (a *App) saveEntity(entityType string, data map[string]any) error {
	id := getString(data, "id")
	if id == "" {
		return fmt.Errorf("ID is required")
	}

	switch entityType {
	case "students":
		fn := getString(data, "first_name")
		ln := getString(data, "last_name")
		if fn == "" || ln == "" {
			return fmt.Errorf("first_name and last_name are required")
		}
		_, err := a.DB.Exec(
			`INSERT OR REPLACE INTO students (id, first_name, last_name, grade, school, parent_id, email, phone, address, notes,
			 dob, birthplace, years_in_us, first_language, previous_schools, courses_outside, active, deleted)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, fn, ln, getString(data, "grade"), getString(data, "school"),
			getString(data, "parent_id"), getString(data, "email"), getString(data, "phone"),
			getString(data, "address"), getString(data, "notes"),
			getString(data, "dob"), getString(data, "birthplace"), getString(data, "years_in_us"),
			getString(data, "first_language"), getString(data, "previous_schools"), getString(data, "courses_outside"),
			boolToInt(getBool(data, "active", true)), boolToInt(getBool(data, "deleted", false)),
		)
		return err

	case "parents":
		fn := getString(data, "first_name")
		ln := getString(data, "last_name")
		if fn == "" || ln == "" {
			return fmt.Errorf("first_name and last_name are required")
		}
		_, err := a.DB.Exec(
			`INSERT OR REPLACE INTO parents (id, first_name, last_name, email, phone, email2, phone2, address, notes, deleted)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, fn, ln, getString(data, "email"), getString(data, "phone"),
			getString(data, "email2"), getString(data, "phone2"),
			getString(data, "address"), getString(data, "notes"),
			boolToInt(getBool(data, "deleted", false)),
		)
		return err

	case "teachers":
		fn := getString(data, "first_name")
		ln := getString(data, "last_name")
		if fn == "" || ln == "" {
			return fmt.Errorf("first_name and last_name are required")
		}
		subjects := getString(data, "subjects")
		_, err := a.DB.Exec(
			`INSERT OR REPLACE INTO teachers (id, first_name, last_name, email, phone, address, subjects, active, deleted)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, fn, ln, getString(data, "email"), getString(data, "phone"),
			getString(data, "address"), subjects,
			boolToInt(getBool(data, "active", true)), boolToInt(getBool(data, "deleted", false)),
		)
		return err

	case "rooms":
		name := getString(data, "name")
		if name == "" {
			return fmt.Errorf("name is required")
		}
		_, err := a.DB.Exec(
			`INSERT OR REPLACE INTO rooms (id, name, capacity, notes, deleted) VALUES (?, ?, ?, ?, ?)`,
			id, name, getInt(data, "capacity"), getString(data, "notes"),
			boolToInt(getBool(data, "deleted", false)),
		)
		return err

	case "schedules":
		dow := getString(data, "day_of_week")
		st := getString(data, "start_time")
		et := getString(data, "end_time")
		if dow == "" || st == "" || et == "" {
			return fmt.Errorf("day_of_week, start_time, and end_time are required")
		}
		studentIDs := getString(data, "student_ids")
		_, err := a.DB.Exec(
			`INSERT OR REPLACE INTO schedules (id, day_of_week, start_time, end_time, teacher_id, room_id, subject, student_ids, effective_from, effective_until, deleted)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, dow, st, et, getString(data, "teacher_id"), getString(data, "room_id"),
			getString(data, "subject"), studentIDs,
			getString(data, "effective_from"), getString(data, "effective_until"),
			boolToInt(getBool(data, "deleted", false)),
		)
		return err

	default:
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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

// HandleExportCSV exports a single entity type as CSV.
func (a *App) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("type")
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s.csv", entity)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	writer := csv.NewWriter(w)

	switch entity {
	case "students":
		writer.Write([]string{"id", "first_name", "last_name", "grade", "school", "parent_id", "email", "phone", "address", "notes", "active"})
		for _, s := range data.Students {
			writer.Write([]string{s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Email, s.Phone, s.Address, s.Notes, boolStr(s.Active)})
		}
	case "parents":
		writer.Write([]string{"id", "first_name", "last_name", "email", "phone", "address", "notes"})
		for _, p := range data.Parents {
			writer.Write([]string{p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Address, p.Notes})
		}
	case "teachers":
		writer.Write([]string{"id", "first_name", "last_name", "email", "phone", "address", "subjects", "active"})
		for _, t := range data.Teachers {
			writer.Write([]string{t.ID, t.FirstName, t.LastName, t.Email, t.Phone, t.Address, strings.Join(t.Subjects, ";"), boolStr(t.Active)})
		}
	case "rooms":
		writer.Write([]string{"id", "name", "capacity", "notes"})
		for _, r := range data.Rooms {
			writer.Write([]string{r.ID, r.Name, fmt.Sprint(r.Capacity), r.Notes})
		}
	case "schedules":
		writer.Write([]string{"id", "day_of_week", "start_time", "end_time", "teacher_id", "room_id", "subject", "student_ids", "effective_from", "effective_until"})
		for _, s := range data.Schedules {
			writer.Write([]string{s.ID, s.DayOfWeek, s.StartTime, s.EndTime, s.TeacherID, s.RoomID, s.Subject, strings.Join(s.StudentIDs, ";"), s.EffectiveFrom, s.EffectiveUntil})
		}
	default:
		http.Error(w, "Unknown entity type", http.StatusBadRequest)
		return
	}
	writer.Flush()
}

// HandleExportCSVZip exports all entity data as a ZIP of CSV files.
func (a *App) HandleExportCSVZip(w http.ResponseWriter, r *http.Request) {
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	csvFiles := map[string]func(*csv.Writer){
		"students.csv": func(cw *csv.Writer) {
			cw.Write([]string{"id", "first_name", "last_name", "grade", "school", "parent_id", "email", "phone", "address", "notes", "active"})
			for _, s := range data.Students {
				cw.Write([]string{s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Email, s.Phone, s.Address, s.Notes, boolStr(s.Active)})
			}
		},
		"parents.csv": func(cw *csv.Writer) {
			cw.Write([]string{"id", "first_name", "last_name", "email", "phone", "address", "notes"})
			for _, p := range data.Parents {
				cw.Write([]string{p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Address, p.Notes})
			}
		},
		"teachers.csv": func(cw *csv.Writer) {
			cw.Write([]string{"id", "first_name", "last_name", "email", "phone", "address", "subjects", "active"})
			for _, t := range data.Teachers {
				cw.Write([]string{t.ID, t.FirstName, t.LastName, t.Email, t.Phone, t.Address, strings.Join(t.Subjects, ";"), boolStr(t.Active)})
			}
		},
		"rooms.csv": func(cw *csv.Writer) {
			cw.Write([]string{"id", "name", "capacity", "notes"})
			for _, rm := range data.Rooms {
				cw.Write([]string{rm.ID, rm.Name, fmt.Sprint(rm.Capacity), rm.Notes})
			}
		},
		"schedules.csv": func(cw *csv.Writer) {
			cw.Write([]string{"id", "day_of_week", "start_time", "end_time", "teacher_id", "room_id", "subject", "student_ids", "effective_from", "effective_until"})
			for _, s := range data.Schedules {
				cw.Write([]string{s.ID, s.DayOfWeek, s.StartTime, s.EndTime, s.TeacherID, s.RoomID, s.Subject, strings.Join(s.StudentIDs, ";"), s.EffectiveFrom, s.EffectiveUntil})
			}
		},
	}

	for name, writeFn := range csvFiles {
		f, err := zw.Create(name)
		if err != nil {
			http.Error(w, "Export error", http.StatusInternalServerError)
			return
		}
		cw := csv.NewWriter(f)
		writeFn(cw)
		cw.Flush()
	}
	zw.Close()

	filename := fmt.Sprintf("classgo-csv-%s.zip", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(buf.Bytes())
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// HandleImportData re-reads data files and imports into DB.
func (a *App) HandleImportData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entityData, err := datastore.ReadAll(a.DataDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("Read failed: %v", err)})
		return
	}
	if err := datastore.ImportAll(a.DB, entityData); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("Import failed: %v", err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": fmt.Sprintf("Imported %d students, %d parents, %d teachers, %d rooms, %d schedules", len(entityData.Students), len(entityData.Parents), len(entityData.Teachers), len(entityData.Rooms), len(entityData.Schedules)),
	})
}

// HandlePasswordReset creates/resets a Memos user account for a student/parent/teacher.
func (a *App) HandlePasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.MemosStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "Memos not configured"})
		return
	}

	var req struct {
		Type     string `json:"type"`     // "students", "parents", "teachers"
		ID       string `json:"id"`       // entity ID
		Password string `json:"password"` // new password
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.ID == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "ID and password are required"})
		return
	}
	if len(req.Password) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Password must be at least 4 characters"})
		return
	}

	// Look up the entity to get name and email
	var firstName, lastName, email string
	switch req.Type {
	case "students":
		err := a.DB.QueryRow("SELECT first_name, last_name, COALESCE(email,'') FROM students WHERE id = ?", req.ID).Scan(&firstName, &lastName, &email)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Student not found"})
			return
		}
	case "parents":
		err := a.DB.QueryRow("SELECT first_name, last_name, COALESCE(email,'') FROM parents WHERE id = ?", req.ID).Scan(&firstName, &lastName, &email)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Parent not found"})
			return
		}
	case "teachers":
		err := a.DB.QueryRow("SELECT first_name, last_name, COALESCE(email,'') FROM teachers WHERE id = ?", req.ID).Scan(&firstName, &lastName, &email)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Teacher not found"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Type must be students, parents, or teachers"})
		return
	}

	// Username is the entity ID (lowercase, unique)
	username := strings.ToLower(req.ID)
	nickname := firstName + " " + lastName

	// Try to create or find existing user, then reset password
	if _, err := memos.EnsureUser(a.MemosStore, username, nickname, email, req.Password); err != nil {
		// User might already exist — try resetting password directly
		log.Printf("EnsureUser for %q: %v (will try password reset)", username, err)
	}

	if err := memos.ResetPassword(a.MemosStore, username, req.Password); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("Password reset failed: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"message":  fmt.Sprintf("Password set for %s (%s)", nickname, username),
		"username": username,
	})
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
