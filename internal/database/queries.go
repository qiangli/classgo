package database

import (
	"database/sql"
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
		`SELECT id, first_name, last_name, grade, school FROM students
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
		if err := rows.Scan(&s.ID, &s.FirstName, &s.LastName, &grade, &school); err != nil {
			return nil, err
		}
		s.Grade = grade.String
		s.School = school.String
		s.Active = true
		students = append(students, s)
	}
	return students, rows.Err()
}
