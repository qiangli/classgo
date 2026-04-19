package database

import (
	"database/sql"
	"time"

	"classgo/internal/models"
)

func TodayAttendees(db *sql.DB) ([]models.Attendance, error) {
	rows, err := db.Query(
		"SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance WHERE date(sign_in_time) = date('now','localtime') ORDER BY sign_in_time DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendees []models.Attendance
	for rows.Next() {
		var a models.Attendance
		var signIn string
		var signOut sql.NullString
		if err := rows.Scan(&a.ID, &a.StudentName, &a.DeviceType, &signIn, &signOut); err != nil {
			return nil, err
		}
		a.SignInTime, _ = models.ParseTimestamp(signIn)
		a.SignInTimeStr = a.SignInTime.Format("3:04 PM")
		a.SignInRaw = a.SignInTime.Format(time.RFC3339)
		if signOut.Valid {
			t, _ := models.ParseTimestamp(signOut.String)
			a.SignOutTime = &t
			a.SignOutTimeStr = t.Format("3:04 PM")
			a.SignOutRaw = t.Format(time.RFC3339)
			dur := t.Sub(a.SignInTime)
			a.Duration = models.FormatDuration(dur)
			a.DurationMinutes = dur.Minutes()
		}
		attendees = append(attendees, a)
	}
	return attendees, rows.Err()
}
