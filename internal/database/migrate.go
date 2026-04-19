package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func OpenDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func MigrateDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS attendance (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		student_name TEXT NOT NULL,
		device_type  TEXT NOT NULL CHECK(device_type IN ('mobile','kiosk')),
		sign_in_time DATETIME DEFAULT (datetime('now','localtime')),
		sign_out_time DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(date(sign_in_time));

	CREATE TABLE IF NOT EXISTS students (
		id         TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		grade      TEXT,
		school     TEXT,
		parent_id  TEXT,
		notes      TEXT,
		active     INTEGER NOT NULL DEFAULT 1,
		row_hash   TEXT
	);

	CREATE TABLE IF NOT EXISTS parents (
		id         TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		email      TEXT,
		phone      TEXT,
		notes      TEXT,
		row_hash   TEXT
	);

	CREATE TABLE IF NOT EXISTS teachers (
		id         TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		email      TEXT,
		phone      TEXT,
		subjects   TEXT,
		active     INTEGER NOT NULL DEFAULT 1,
		row_hash   TEXT
	);

	CREATE TABLE IF NOT EXISTS rooms (
		id       TEXT PRIMARY KEY,
		name     TEXT NOT NULL,
		capacity INTEGER,
		notes    TEXT,
		row_hash TEXT
	);

	CREATE TABLE IF NOT EXISTS schedules (
		id              TEXT PRIMARY KEY,
		day_of_week     TEXT NOT NULL,
		start_time      TEXT NOT NULL,
		end_time        TEXT NOT NULL,
		teacher_id      TEXT,
		room_id         TEXT,
		subject         TEXT,
		student_ids     TEXT,
		effective_from  TEXT,
		effective_until TEXT,
		row_hash        TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_schedules_day ON schedules(day_of_week);
	CREATE INDEX IF NOT EXISTS idx_schedules_room ON schedules(room_id);
	CREATE INDEX IF NOT EXISTS idx_schedules_teacher ON schedules(teacher_id);

	CREATE TABLE IF NOT EXISTS attendance_meta (
		attendance_id INTEGER PRIMARY KEY,
		student_id    TEXT,
		schedule_id   TEXT
	);

	CREATE TABLE IF NOT EXISTS import_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		filename    TEXT NOT NULL,
		file_hash   TEXT NOT NULL,
		imported_at DATETIME DEFAULT (datetime('now','localtime')),
		row_count   INTEGER
	);
	`
	_, err := db.Exec(schema)
	return err
}

// DropIndexTables drops all spreadsheet-derived index tables for a full rebuild.
func DropIndexTables(db *sql.DB) error {
	tables := []string{"students", "parents", "teachers", "rooms", "schedules", "attendance_meta", "import_log"}
	for _, t := range tables {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return err
		}
	}
	return nil
}
