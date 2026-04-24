package reports

import (
	"database/sql"
)

// GetSubscriptions returns all report subscriptions for a user.
func GetSubscriptions(db *sql.DB, userID, userType string) ([]ReportSubscription, error) {
	rows, err := db.Query(
		`SELECT id, user_id, user_type, report_type, frequency, day_of_week, channel, active,
		        COALESCE(created_at,''), COALESCE(updated_at,'')
		 FROM report_subscriptions WHERE user_id = ? AND user_type = ?
		 ORDER BY created_at DESC`, userID, userType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []ReportSubscription
	for rows.Next() {
		var s ReportSubscription
		rows.Scan(&s.ID, &s.UserID, &s.UserType, &s.ReportType, &s.Frequency,
			&s.DayOfWeek, &s.Channel, &s.Active, &s.CreatedAt, &s.UpdatedAt)
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// CreateSubscription creates a new report subscription.
func CreateSubscription(db *sql.DB, sub ReportSubscription) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO report_subscriptions (user_id, user_type, report_type, frequency, day_of_week, channel)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sub.UserID, sub.UserType, sub.ReportType, sub.Frequency, sub.DayOfWeek, sub.Channel)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateSubscription updates an existing subscription's frequency, day, channel, or active status.
func UpdateSubscription(db *sql.DB, id int, frequency, dayOfWeek, channel string, active bool) error {
	_, err := db.Exec(
		`UPDATE report_subscriptions SET frequency = ?, day_of_week = ?, channel = ?, active = ?,
		        updated_at = datetime('now','localtime')
		 WHERE id = ?`,
		frequency, dayOfWeek, channel, active, id)
	return err
}

// DeleteSubscription removes a subscription.
func DeleteSubscription(db *sql.DB, id int, userID, userType string) error {
	_, err := db.Exec(
		"DELETE FROM report_subscriptions WHERE id = ? AND user_id = ? AND user_type = ?",
		id, userID, userType)
	return err
}

// GetActiveSubscriptions returns all active subscriptions, optionally filtered by frequency.
func GetActiveSubscriptions(db *sql.DB) ([]ReportSubscription, error) {
	rows, err := db.Query(
		`SELECT id, user_id, user_type, report_type, frequency, day_of_week, channel, active,
		        COALESCE(created_at,''), COALESCE(updated_at,'')
		 FROM report_subscriptions WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []ReportSubscription
	for rows.Next() {
		var s ReportSubscription
		rows.Scan(&s.ID, &s.UserID, &s.UserType, &s.ReportType, &s.Frequency,
			&s.DayOfWeek, &s.Channel, &s.Active, &s.CreatedAt, &s.UpdatedAt)
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
