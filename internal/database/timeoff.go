package database

import (
	"database/sql"

	"classgo/internal/models"
)

// CreateTimeOff inserts a new time-off record. Returns the new row ID.
func CreateTimeOff(db *sql.DB, userID, userType, date, typ, scheduleType string, hours float64, notes, createdBy string) (int64, error) {
	res, err := db.Exec(
		`INSERT OR REPLACE INTO timeoff (user_id, user_type, date, type, schedule_type, hours, notes, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, userType, date, typ, scheduleType, hours, notes, createdBy,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateTimeOff updates an existing time-off record by ID.
func UpdateTimeOff(db *sql.DB, id int, typ, scheduleType string, hours float64, notes string) error {
	_, err := db.Exec(
		`UPDATE timeoff SET type = ?, schedule_type = ?, hours = ?, notes = ? WHERE id = ?`,
		typ, scheduleType, hours, notes, id,
	)
	return err
}

// DeleteTimeOff hard-deletes a time-off record by ID.
func DeleteTimeOff(db *sql.DB, id int) error {
	_, err := db.Exec(`DELETE FROM timeoff WHERE id = ?`, id)
	return err
}

// ListTimeOff returns time-off records filtered by user and date range.
// Pass empty strings to skip a filter.
func ListTimeOff(db *sql.DB, userID, userType, fromDate, toDate string) ([]models.TimeOff, error) {
	q := `SELECT id, user_id, user_type, date, type, schedule_type, hours, COALESCE(notes,''), COALESCE(created_by,''), COALESCE(created_at,'') FROM timeoff WHERE 1=1`
	var args []any
	if userID != "" {
		q += " AND user_id = ?"
		args = append(args, userID)
	}
	if userType != "" {
		q += " AND user_type = ?"
		args = append(args, userType)
	}
	if fromDate != "" {
		q += " AND date >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		q += " AND date <= ?"
		args = append(args, toDate)
	}
	q += " ORDER BY date DESC, user_id"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TimeOff
	for rows.Next() {
		var t models.TimeOff
		if err := rows.Scan(&t.ID, &t.UserID, &t.UserType, &t.Date, &t.Type, &t.ScheduleType, &t.Hours, &t.Notes, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
