package database

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"classgo/internal/models"
)

// InsertAudit logs a check-in or check-out event with device identity.
func InsertAudit(db *sql.DB, audit models.CheckinAudit) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO checkin_audit (attendance_id, student_name, student_id, device_type,
		 client_ip, fingerprint, device_id, action)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		audit.AttendanceID, audit.StudentName, audit.StudentID, audit.DeviceType,
		audit.ClientIP, audit.Fingerprint, audit.DeviceID, audit.Action,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// FlagSuspiciousCheckins detects and flags buddy-punching patterns.
// Rule 1: Same device triple checked in 2+ different students within 5 minutes.
// Rule 2: Same device triple checked in 3+ different students in one day.
func FlagSuspiciousCheckins(db *sql.DB, auditID int64, clientIP, fingerprint, deviceID, studentName string) {
	deviceKey := clientIP
	if fingerprint != "" {
		deviceKey += ":" + fingerprint
	}
	if deviceID != "" {
		deviceKey += ":" + deviceID
	}

	// Rule 1: 2+ different students from same device in last 5 minutes
	var recentDiff int
	db.QueryRow(`
		SELECT COUNT(DISTINCT student_name) FROM checkin_audit
		WHERE client_ip = ? AND COALESCE(fingerprint,'') = COALESCE(?,'') AND COALESCE(device_id,'') = COALESCE(?,'')
		AND action = 'checkin'
		AND created_at >= datetime('now','localtime','-5 minutes')
		AND student_name != ?`,
		clientIP, fingerprint, deviceID, studentName,
	).Scan(&recentDiff)

	if recentDiff >= 1 {
		db.Exec("UPDATE checkin_audit SET flagged = 1, flag_reason = ? WHERE id = ?",
			"Same device checked in multiple students within 5 minutes", auditID)
		return
	}

	// Rule 2: 3+ different students from same device today
	var dailyDiff int
	db.QueryRow(`
		SELECT COUNT(DISTINCT student_name) FROM checkin_audit
		WHERE client_ip = ? AND COALESCE(fingerprint,'') = COALESCE(?,'') AND COALESCE(device_id,'') = COALESCE(?,'')
		AND action = 'checkin'
		AND date(created_at) = date('now','localtime')`,
		clientIP, fingerprint, deviceID,
	).Scan(&dailyDiff)

	if dailyDiff >= 3 {
		db.Exec("UPDATE checkin_audit SET flagged = 1, flag_reason = ? WHERE id = ?",
			"Same device checked in 3+ different students today", auditID)
	}
}

// GetFlaggedAudits returns flagged check-in audit records for a date range.
func GetFlaggedAudits(db *sql.DB, from, to string) ([]models.CheckinAudit, error) {
	rows, err := db.Query(`
		SELECT id, COALESCE(attendance_id,0), student_name, COALESCE(student_id,''), device_type,
		       client_ip, COALESCE(fingerprint,''), COALESCE(device_id,''), action,
		       COALESCE(created_at,''), flagged, COALESCE(flag_reason,'')
		FROM checkin_audit
		WHERE (flagged = 1 OR date(created_at) BETWEEN ? AND ?)
		AND flagged = 1
		ORDER BY created_at DESC`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAudits(rows)
}

// GetDeviceSummary returns per-device check-in counts for a date.
func GetDeviceSummary(db *sql.DB, date string) ([]map[string]any, error) {
	rows, err := db.Query(`
		SELECT client_ip, COALESCE(fingerprint,'') as fp, COALESCE(device_id,'') as did,
		       COUNT(*) as total_checkins, COUNT(DISTINCT student_name) as unique_students,
		       GROUP_CONCAT(DISTINCT student_name) as students
		FROM checkin_audit
		WHERE action = 'checkin' AND date(created_at) = ?
		GROUP BY client_ip, fp, did
		ORDER BY unique_students DESC`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var ip, fp, did, students string
		var total, unique int
		if err := rows.Scan(&ip, &fp, &did, &total, &unique, &students); err != nil {
			continue
		}
		results = append(results, map[string]any{
			"client_ip":       ip,
			"fingerprint":     fp,
			"device_id":       did,
			"total_checkins":  total,
			"unique_students": unique,
			"students":        students,
		})
	}
	return results, nil
}

// DismissAuditFlag marks an audit flag as reviewed.
func DismissAuditFlag(db *sql.DB, id int) error {
	_, err := db.Exec("UPDATE checkin_audit SET flagged = 0, flag_reason = 'dismissed' WHERE id = ?", id)
	return err
}

// GetStudentPinHash returns the pin_hash for a student (legacy, unused for flagged students).
func GetStudentPinHash(db *sql.DB, studentID string) (string, error) {
	var hash sql.NullString
	err := db.QueryRow("SELECT pin_hash FROM students WHERE id = ?", studentID).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash.String, nil
}

// SetStudentPinHash sets the pin_hash for a student (legacy).
func SetStudentPinHash(db *sql.DB, studentID, hash string) error {
	_, err := db.Exec("UPDATE students SET pin_hash = ? WHERE id = ?", hash, studentID)
	return err
}

// GenerateStudentPin generates a random 4-digit PIN for a student, stores it with today's date, and returns it.
func GenerateStudentPin(db *sql.DB, studentID string) (string, error) {
	pin := fmt.Sprintf("%04d", rand.Intn(10000))
	today := time.Now().Format("2006-01-02")
	_, err := db.Exec("UPDATE students SET personal_pin = ?, pin_generated_date = ? WHERE id = ?", pin, today, studentID)
	if err != nil {
		return "", err
	}
	return pin, nil
}

// EnsureDailyStudentPin returns the current PIN for a student, regenerating if the date has changed.
func EnsureDailyStudentPin(db *sql.DB, studentID string) (string, error) {
	var pin, genDate sql.NullString
	err := db.QueryRow("SELECT personal_pin, pin_generated_date FROM students WHERE id = ?", studentID).Scan(&pin, &genDate)
	if err != nil {
		return "", err
	}
	today := time.Now().Format("2006-01-02")
	if pin.String == "" || genDate.String != today {
		return GenerateStudentPin(db, studentID)
	}
	return pin.String, nil
}

// GetStudentPin returns the current personal_pin and pin_generated_date for a student.
func GetStudentPin(db *sql.DB, studentID string) (string, string, error) {
	var pin, genDate sql.NullString
	err := db.QueryRow("SELECT personal_pin, pin_generated_date FROM students WHERE id = ?", studentID).Scan(&pin, &genDate)
	if err != nil {
		return "", "", err
	}
	return pin.String, genDate.String, nil
}

// ResetStudentPin regenerates the personal PIN for a student.
func ResetStudentPin(db *sql.DB, studentID string) error {
	_, err := GenerateStudentPin(db, studentID)
	return err
}

// StudentRequiresPIN checks if a specific student has the require_pin flag set.
func StudentRequiresPIN(db *sql.DB, studentID string) bool {
	var req int
	err := db.QueryRow("SELECT COALESCE(require_pin, 0) FROM students WHERE id = ?", studentID).Scan(&req)
	return err == nil && req == 1
}

// SetStudentRequirePIN sets or clears the require_pin flag for a student.
func SetStudentRequirePIN(db *sql.DB, studentID string, require bool) error {
	v := 0
	if require {
		v = 1
	}
	_, err := db.Exec("UPDATE students SET require_pin = ? WHERE id = ?", v, studentID)
	return err
}

func scanAudits(rows *sql.Rows) ([]models.CheckinAudit, error) {
	var audits []models.CheckinAudit
	for rows.Next() {
		var a models.CheckinAudit
		if err := rows.Scan(&a.ID, &a.AttendanceID, &a.StudentName, &a.StudentID, &a.DeviceType,
			&a.ClientIP, &a.Fingerprint, &a.DeviceID, &a.Action,
			&a.CreatedAt, &a.Flagged, &a.FlagReason); err != nil {
			return nil, err
		}
		audits = append(audits, a)
	}
	return audits, rows.Err()
}
