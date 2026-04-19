package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func OpenDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func MigrateDB(db *sql.DB) error {
	// Migrate old column names if they exist
	migrateColumns(db)
	// Add new columns to existing tables
	addMissingColumns(db)

	schema := `
	CREATE TABLE IF NOT EXISTS attendance (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		student_name TEXT NOT NULL,
		device_type  TEXT NOT NULL CHECK(device_type IN ('mobile','kiosk')),
		check_in_time DATETIME DEFAULT (datetime('now','localtime')),
		check_out_time DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(date(check_in_time));

	CREATE TABLE IF NOT EXISTS students (
		id          TEXT PRIMARY KEY,
		first_name  TEXT NOT NULL,
		last_name   TEXT NOT NULL,
		grade       TEXT,
		school      TEXT,
		parent_id   TEXT,
		email       TEXT,
		phone       TEXT,
		address     TEXT,
		notes       TEXT,
		active      INTEGER NOT NULL DEFAULT 1,
		deleted     INTEGER NOT NULL DEFAULT 0,
		row_hash    TEXT,
		pin_hash    TEXT,
		require_pin INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS parents (
		id         TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		email      TEXT,
		phone      TEXT,
		address    TEXT,
		notes      TEXT,
		deleted    INTEGER NOT NULL DEFAULT 0,
		row_hash   TEXT
	);

	CREATE TABLE IF NOT EXISTS teachers (
		id         TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		email      TEXT,
		phone      TEXT,
		address    TEXT,
		subjects   TEXT,
		active     INTEGER NOT NULL DEFAULT 1,
		deleted    INTEGER NOT NULL DEFAULT 0,
		row_hash   TEXT
	);

	CREATE TABLE IF NOT EXISTS rooms (
		id       TEXT PRIMARY KEY,
		name     TEXT NOT NULL,
		capacity INTEGER,
		notes    TEXT,
		deleted  INTEGER NOT NULL DEFAULT 0,
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
		deleted         INTEGER NOT NULL DEFAULT 0,
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

	CREATE TABLE IF NOT EXISTS tracker_items (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		notes      TEXT,
		start_date TEXT,
		due_date   TEXT,
		priority   TEXT NOT NULL DEFAULT 'medium',
		recurrence TEXT NOT NULL DEFAULT 'daily',
		category   TEXT,
		created_by TEXT NOT NULL DEFAULT 'admin',
		active     INTEGER NOT NULL DEFAULT 1,
		deleted    INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now','localtime')),
		updated_at DATETIME DEFAULT (datetime('now','localtime'))
	);

	CREATE TABLE IF NOT EXISTS student_tracker_items (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id   TEXT NOT NULL,
		name         TEXT NOT NULL,
		notes        TEXT,
		start_date   TEXT,
		due_date     TEXT,
		priority     TEXT NOT NULL DEFAULT 'medium',
		recurrence   TEXT NOT NULL DEFAULT 'none',
		category     TEXT,
		created_by   TEXT,
		owner_type   TEXT NOT NULL DEFAULT 'admin',
		completed        INTEGER NOT NULL DEFAULT 0,
		completed_at     TEXT,
		completed_by     TEXT,
		requires_signoff INTEGER NOT NULL DEFAULT 1,
		active           INTEGER NOT NULL DEFAULT 1,
		deleted      INTEGER NOT NULL DEFAULT 0,
		created_at   DATETIME DEFAULT (datetime('now','localtime')),
		updated_at   DATETIME DEFAULT (datetime('now','localtime'))
	);
	CREATE INDEX IF NOT EXISTS idx_sti_student ON student_tracker_items(student_id);
	CREATE INDEX IF NOT EXISTS idx_sti_created_by ON student_tracker_items(created_by);

	CREATE TABLE IF NOT EXISTS tracker_responses (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id    TEXT NOT NULL,
		student_name  TEXT NOT NULL,
		item_type     TEXT NOT NULL CHECK(item_type IN ('global','adhoc')),
		item_id       INTEGER NOT NULL,
		item_name     TEXT NOT NULL,
		status        TEXT NOT NULL CHECK(status IN ('done','not_done')),
		notes         TEXT,
		response_date DATE NOT NULL DEFAULT (date('now','localtime')),
		attendance_id INTEGER,
		responded_at  DATETIME DEFAULT (datetime('now','localtime'))
	);
	CREATE INDEX IF NOT EXISTS idx_tr_student_date ON tracker_responses(student_id, response_date);
	CREATE INDEX IF NOT EXISTS idx_tr_attendance ON tracker_responses(attendance_id);

	CREATE TABLE IF NOT EXISTS checkin_audit (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		attendance_id INTEGER,
		student_name  TEXT NOT NULL,
		student_id    TEXT,
		device_type   TEXT NOT NULL,
		client_ip     TEXT NOT NULL,
		fingerprint   TEXT,
		device_id     TEXT,
		action        TEXT NOT NULL CHECK(action IN ('checkin','checkout')),
		created_at    DATETIME DEFAULT (datetime('now','localtime')),
		flagged       INTEGER NOT NULL DEFAULT 0,
		flag_reason   TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_audit_ip_date ON checkin_audit(client_ip, date(created_at));
	CREATE INDEX IF NOT EXISTS idx_audit_flagged ON checkin_audit(flagged) WHERE flagged = 1;
	CREATE INDEX IF NOT EXISTS idx_audit_device ON checkin_audit(client_ip, fingerprint, device_id, date(created_at));
	`
	_, err := db.Exec(schema)
	return err
}

// migrateColumns renames sign_in_time/sign_out_time to check_in_time/check_out_time
// in existing databases. Safe to call on databases that already have the new names.
func migrateColumns(db *sql.DB) {
	// Check if old column names exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('attendance') WHERE name = 'sign_in_time'").Scan(&count)
	if err != nil || count == 0 {
		return
	}
	db.Exec("ALTER TABLE attendance RENAME COLUMN sign_in_time TO check_in_time")
	db.Exec("ALTER TABLE attendance RENAME COLUMN sign_out_time TO check_out_time")
	db.Exec("DROP INDEX IF EXISTS idx_attendance_date")
}

// addMissingColumns adds columns that were introduced after initial schema creation.
// ALTER TABLE ADD COLUMN is safe to run on columns that already exist — SQLite returns
// an error which we simply ignore.
func addMissingColumns(db *sql.DB) {
	alters := []string{
		"ALTER TABLE students ADD COLUMN email TEXT",
		"ALTER TABLE students ADD COLUMN phone TEXT",
		"ALTER TABLE students ADD COLUMN address TEXT",
		"ALTER TABLE parents ADD COLUMN address TEXT",
		"ALTER TABLE teachers ADD COLUMN address TEXT",
		"ALTER TABLE students ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE parents ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE teachers ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE rooms ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE schedules ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0",
		// Tracker item enhancements
		"ALTER TABLE tracker_items ADD COLUMN start_date TEXT",
		"ALTER TABLE tracker_items ADD COLUMN due_date TEXT",
		"ALTER TABLE tracker_items ADD COLUMN priority TEXT NOT NULL DEFAULT 'medium'",
		"ALTER TABLE tracker_items ADD COLUMN recurrence TEXT NOT NULL DEFAULT 'daily'",
		"ALTER TABLE tracker_items ADD COLUMN category TEXT",
		"ALTER TABLE tracker_items ADD COLUMN created_by TEXT NOT NULL DEFAULT 'admin'",
		"ALTER TABLE tracker_items ADD COLUMN updated_at DATETIME",
		// Student tracker item enhancements
		"ALTER TABLE student_tracker_items ADD COLUMN notes TEXT",
		"ALTER TABLE student_tracker_items ADD COLUMN start_date TEXT",
		"ALTER TABLE student_tracker_items ADD COLUMN due_date TEXT",
		"ALTER TABLE student_tracker_items ADD COLUMN priority TEXT NOT NULL DEFAULT 'medium'",
		"ALTER TABLE student_tracker_items ADD COLUMN recurrence TEXT NOT NULL DEFAULT 'none'",
		"ALTER TABLE student_tracker_items ADD COLUMN category TEXT",
		"ALTER TABLE student_tracker_items ADD COLUMN owner_type TEXT NOT NULL DEFAULT 'admin'",
		"ALTER TABLE student_tracker_items ADD COLUMN completed INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE student_tracker_items ADD COLUMN completed_at TEXT",
		"ALTER TABLE student_tracker_items ADD COLUMN completed_by TEXT",
		"ALTER TABLE student_tracker_items ADD COLUMN updated_at DATETIME",
		// Tracker response enhancements
		"ALTER TABLE tracker_responses ADD COLUMN notes TEXT",
		// Sign-off requirement per task item
		"ALTER TABLE student_tracker_items ADD COLUMN requires_signoff INTEGER NOT NULL DEFAULT 1",
		// Per-student PIN
		"ALTER TABLE students ADD COLUMN pin_hash TEXT",
		"ALTER TABLE students ADD COLUMN require_pin INTEGER NOT NULL DEFAULT 0",
	}
	for _, stmt := range alters {
		db.Exec(stmt) // ignore "duplicate column" errors
	}
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
