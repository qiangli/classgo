package database

import (
	"database/sql"
	"strings"
	"time"

	"classgo/internal/models"
)

func TodayAttendees(db *sql.DB) ([]models.Attendance, error) {
	rows, err := db.Query(
		"SELECT id, student_name, device_type, check_in_time, check_out_time FROM attendance WHERE date(check_in_time) = date('now','localtime') ORDER BY check_in_time DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendees []models.Attendance
	for rows.Next() {
		var a models.Attendance
		var checkIn string
		var checkOut sql.NullString
		if err := rows.Scan(&a.ID, &a.StudentName, &a.DeviceType, &checkIn, &checkOut); err != nil {
			return nil, err
		}
		a.CheckInTime, _ = models.ParseTimestamp(checkIn)
		a.CheckInTimeStr = a.CheckInTime.Format("3:04 PM")
		a.CheckInRaw = a.CheckInTime.Format(time.RFC3339)
		a.Date = a.CheckInTime.Format("2006-01-02")
		if checkOut.Valid {
			t, _ := models.ParseTimestamp(checkOut.String)
			a.CheckOutTime = &t
			a.CheckOutTimeStr = t.Format("3:04 PM")
			a.CheckOutRaw = t.Format(time.RFC3339)
			dur := t.Sub(a.CheckInTime)
			a.Duration = models.FormatDuration(dur)
			a.DurationMinutes = dur.Minutes()
		}
		attendees = append(attendees, a)
	}
	return attendees, rows.Err()
}

// AttendeesByDateRange returns attendance records within a date range (inclusive).
// Dates should be in YYYY-MM-DD format. If from is empty, defaults to today.
// Optional filters: studentID, teacherID, parentID filter via attendance_meta joins.
func AttendeesByDateRange(db *sql.DB, from, to, studentID, teacherID, parentID string) ([]models.Attendance, error) {
	q := "SELECT DISTINCT a.id, a.student_name, a.device_type, a.check_in_time, a.check_out_time FROM attendance a"
	var joins []string
	var wheres []string
	var args []any

	needMeta := studentID != "" || teacherID != "" || parentID != ""
	if needMeta {
		joins = append(joins, "LEFT JOIN attendance_meta am ON am.attendance_id = a.id")
	}
	if studentID != "" {
		wheres = append(wheres, "am.student_id = ?")
		args = append(args, studentID)
	}
	if teacherID != "" {
		joins = append(joins, "LEFT JOIN schedules sch ON sch.id = am.schedule_id")
		wheres = append(wheres, "sch.teacher_id = ?")
		args = append(args, teacherID)
	}
	if parentID != "" {
		joins = append(joins, "LEFT JOIN students st ON st.id = am.student_id")
		wheres = append(wheres, "st.parent_id = ?")
		args = append(args, parentID)
	}

	for _, j := range joins {
		q += " " + j
	}

	if from != "" && to != "" {
		wheres = append(wheres, "date(a.check_in_time) >= ?", "date(a.check_in_time) <= ?")
		args = append(args, from, to)
	} else if from != "" {
		wheres = append(wheres, "date(a.check_in_time) >= ?")
		args = append(args, from)
	} else {
		wheres = append(wheres, "date(a.check_in_time) = date('now','localtime')")
	}

	if len(wheres) > 0 {
		q += " WHERE " + strings.Join(wheres, " AND ")
	}
	q += " ORDER BY a.check_in_time DESC"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAttendees(rows)
}

// AttendanceMetrics returns summary statistics for a date range.
type AttendanceMetrics struct {
	TotalCheckIns   int         `json:"total_checkins"`
	UniqueStudents  int         `json:"unique_students"`
	TotalCheckOuts  int         `json:"total_checkouts"`
	AvgDurationMins float64     `json:"avg_duration_mins"`
	DayCount        int         `json:"day_count"`
	ByDay           []DayMetric `json:"by_day"`
}

type DayMetric struct {
	Date      string  `json:"date"`
	CheckIns  int     `json:"checkins"`
	CheckOuts int     `json:"checkouts"`
	AvgMins   float64 `json:"avg_mins"`
}

func GetAttendanceMetrics(db *sql.DB, from, to string) (*AttendanceMetrics, error) {
	m := &AttendanceMetrics{}

	// Overall stats
	err := db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT student_name), COUNT(check_out_time),
		       COALESCE(AVG(CASE WHEN check_out_time IS NOT NULL
		           THEN (julianday(check_out_time) - julianday(check_in_time)) * 24 * 60
		           ELSE NULL END), 0),
		       COUNT(DISTINCT date(check_in_time))
		FROM attendance WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?`,
		from, to,
	).Scan(&m.TotalCheckIns, &m.UniqueStudents, &m.TotalCheckOuts, &m.AvgDurationMins, &m.DayCount)
	if err != nil {
		return nil, err
	}

	// Per-day breakdown
	rows, err := db.Query(`
		SELECT date(check_in_time) as d, COUNT(*), COUNT(check_out_time),
		       COALESCE(AVG(CASE WHEN check_out_time IS NOT NULL
		           THEN (julianday(check_out_time) - julianday(check_in_time)) * 24 * 60
		           ELSE NULL END), 0)
		FROM attendance WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?
		GROUP BY d ORDER BY d`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d DayMetric
		if err := rows.Scan(&d.Date, &d.CheckIns, &d.CheckOuts, &d.AvgMins); err != nil {
			continue
		}
		m.ByDay = append(m.ByDay, d)
	}
	return m, nil
}

func scanAttendees(rows *sql.Rows) ([]models.Attendance, error) {
	var attendees []models.Attendance
	for rows.Next() {
		var a models.Attendance
		var checkIn string
		var checkOut sql.NullString
		if err := rows.Scan(&a.ID, &a.StudentName, &a.DeviceType, &checkIn, &checkOut); err != nil {
			return nil, err
		}
		a.CheckInTime, _ = models.ParseTimestamp(checkIn)
		a.CheckInTimeStr = a.CheckInTime.Format("3:04 PM")
		a.CheckInRaw = a.CheckInTime.Format(time.RFC3339)
		a.Date = a.CheckInTime.Format("2006-01-02")
		if checkOut.Valid {
			t, _ := models.ParseTimestamp(checkOut.String)
			a.CheckOutTime = &t
			a.CheckOutTimeStr = t.Format("3:04 PM")
			a.CheckOutRaw = t.Format(time.RFC3339)
			dur := t.Sub(a.CheckInTime)
			a.Duration = models.FormatDuration(dur)
			a.DurationMinutes = dur.Minutes()
		}
		attendees = append(attendees, a)
	}
	return attendees, rows.Err()
}

// SearchStudents searches active students by ID, first name, last name, or full name.
func SearchStudents(db *sql.DB, query string, limit int) ([]models.Student, error) {
	like := "%" + query + "%"
	rows, err := db.Query(
		`SELECT id, first_name, last_name, grade, school, COALESCE(require_pin, 0) FROM students
		 WHERE active = 1 AND (
		   LOWER(id) LIKE LOWER(?) OR
		   LOWER(first_name) LIKE LOWER(?) OR
		   LOWER(last_name) LIKE LOWER(?) OR
		   LOWER(first_name || ' ' || last_name) LIKE LOWER(?)
		 )
		 ORDER BY first_name, last_name LIMIT ?`,
		like, like, like, like, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []models.Student
	for rows.Next() {
		var s models.Student
		var grade, school sql.NullString
		if err := rows.Scan(&s.ID, &s.FirstName, &s.LastName, &grade, &school, &s.RequirePIN); err != nil {
			return nil, err
		}
		s.Grade = grade.String
		s.School = school.String
		s.Active = true
		students = append(students, s)
	}
	return students, rows.Err()
}
