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
