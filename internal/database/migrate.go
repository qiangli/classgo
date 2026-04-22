package database

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

func OpenDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func MigrateDB(db *sql.DB) error {
	// Migrate old column names if they exist
	migrateColumns(db)

	// Rename item_type 'adhoc' -> 'personal' and recreate table constraint
	migrateTrackerItemType(db)
	// Add new columns to existing tables
	addMissingColumns(db)
	// Migrate to unified task_items table
	migrateToTaskItems(db)

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
		id               TEXT PRIMARY KEY,
		first_name       TEXT NOT NULL,
		last_name        TEXT NOT NULL,
		grade            TEXT,
		school           TEXT,
		parent_id        TEXT,
		email            TEXT,
		phone            TEXT,
		address          TEXT,
		notes            TEXT,
		dob              TEXT,
		birthplace       TEXT,
		years_in_us      TEXT,
		first_language   TEXT,
		previous_schools TEXT,
		courses_outside  TEXT,
		profile_status   TEXT NOT NULL DEFAULT '',
		active           INTEGER NOT NULL DEFAULT 1,
		deleted          INTEGER NOT NULL DEFAULT 0,
		row_hash         TEXT,
		pin_hash           TEXT,
		require_pin        INTEGER NOT NULL DEFAULT 0,
		personal_pin       TEXT,
		pin_generated_date TEXT
	);

	CREATE TABLE IF NOT EXISTS parents (
		id         TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		email      TEXT,
		phone      TEXT,
		email2     TEXT,
		phone2     TEXT,
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
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		name            TEXT NOT NULL,
		notes           TEXT,
		start_date      TEXT,
		end_date        TEXT,
		priority        TEXT NOT NULL DEFAULT 'medium',
		recurrence      TEXT NOT NULL DEFAULT 'daily',
		category        TEXT,
		created_by      TEXT NOT NULL DEFAULT 'admin',
		requires_signoff INTEGER NOT NULL DEFAULT 1,
		active          INTEGER NOT NULL DEFAULT 1,
		deleted         INTEGER NOT NULL DEFAULT 0,
		created_at      DATETIME DEFAULT (datetime('now','localtime')),
		updated_at      DATETIME DEFAULT (datetime('now','localtime'))
	);

	CREATE TABLE IF NOT EXISTS student_tracker_items (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id   TEXT NOT NULL,
		name         TEXT NOT NULL,
		notes        TEXT,
		start_date   TEXT,
		end_date     TEXT,
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

	CREATE TABLE IF NOT EXISTS task_items (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		scope           INTEGER NOT NULL DEFAULT 1 CHECK(scope IN (1, 2, 3)),
		schedule_id     TEXT,
		student_id      TEXT,
		type            TEXT NOT NULL DEFAULT 'task',
		name            TEXT NOT NULL,
		notes           TEXT,
		start_date      TEXT,
		end_date        TEXT,
		priority        TEXT NOT NULL DEFAULT 'medium',
		recurrence      TEXT NOT NULL DEFAULT 'daily',
		category        TEXT,
		criteria        TEXT,
		group_id        TEXT,
		group_order     INTEGER,
		created_by      TEXT NOT NULL DEFAULT 'admin',
		owner_type      TEXT NOT NULL DEFAULT 'admin',
		completed       INTEGER NOT NULL DEFAULT 0,
		completed_at    TEXT,
		completed_by    TEXT,
		active          INTEGER NOT NULL DEFAULT 1,
		deleted         INTEGER NOT NULL DEFAULT 0,
		created_at      DATETIME DEFAULT (datetime('now','localtime')),
		updated_at      DATETIME DEFAULT (datetime('now','localtime')),
		legacy_table    TEXT,
		legacy_id       INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_task_items_scope ON task_items(scope);
	CREATE INDEX IF NOT EXISTS idx_task_items_student ON task_items(student_id) WHERE student_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_task_items_schedule ON task_items(schedule_id) WHERE schedule_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_task_items_created_by ON task_items(created_by);
	CREATE INDEX IF NOT EXISTS idx_task_items_group ON task_items(group_id) WHERE group_id IS NOT NULL;

	CREATE TABLE IF NOT EXISTS task_groups (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		min_required   INTEGER,
		enforce_order  INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS tracker_responses (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id    TEXT NOT NULL,
		student_name  TEXT NOT NULL,
		item_type     TEXT NOT NULL CHECK(item_type IN ('global','personal')),
		item_id       INTEGER NOT NULL,
		item_name     TEXT NOT NULL,
		status        TEXT NOT NULL CHECK(status IN ('done','not_done')),
		notes         TEXT,
		response_date DATE NOT NULL DEFAULT (date('now','localtime')),
		attendance_id INTEGER,
		responded_at  DATETIME DEFAULT (datetime('now','localtime')),
		due_date      TEXT,
		is_late       INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_tr_student_date ON tracker_responses(student_id, response_date);
	CREATE INDEX IF NOT EXISTS idx_tr_attendance ON tracker_responses(attendance_id);
	CREATE INDEX IF NOT EXISTS idx_tr_due_date ON tracker_responses(student_id, due_date);

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

	CREATE TABLE IF NOT EXISTS user_preferences (
		user_id    TEXT NOT NULL,
		pref_key   TEXT NOT NULL,
		pref_value TEXT NOT NULL,
		updated_at DATETIME DEFAULT (datetime('now','localtime')),
		PRIMARY KEY (user_id, pref_key)
	);
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

// migrateTrackerItemType renames item_type 'adhoc' to 'personal' in tracker_responses
// and recreates the table to update the CHECK constraint. Also clears any orphaned responses.
func migrateTrackerItemType(db *sql.DB) {
	// Check if migration is needed
	var adhocCount int
	db.QueryRow("SELECT COUNT(*) FROM tracker_responses WHERE item_type = 'adhoc'").Scan(&adhocCount)

	// Check the old CHECK constraint by trying to detect the old schema
	var sql string
	db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='tracker_responses'").Scan(&sql)
	needsRecreate := strings.Contains(sql, "'adhoc'")

	if adhocCount == 0 && !needsRecreate {
		return
	}

	// Recreate table with updated CHECK constraint
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	tx.Exec(`CREATE TABLE IF NOT EXISTS tracker_responses_new (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id    TEXT NOT NULL,
		student_name  TEXT NOT NULL,
		item_type     TEXT NOT NULL CHECK(item_type IN ('global','personal')),
		item_id       INTEGER NOT NULL,
		item_name     TEXT NOT NULL,
		status        TEXT NOT NULL CHECK(status IN ('done','not_done')),
		notes         TEXT,
		response_date DATE NOT NULL DEFAULT (date('now','localtime')),
		attendance_id INTEGER,
		responded_at  DATETIME DEFAULT (datetime('now','localtime'))
	)`)
	tx.Exec(`INSERT INTO tracker_responses_new (id, student_id, student_name, item_type, item_id, item_name, status, notes, response_date, attendance_id, responded_at)
		SELECT id, student_id, student_name, CASE WHEN item_type = 'adhoc' THEN 'personal' ELSE item_type END, item_id, item_name, status, notes, response_date, attendance_id, responded_at
		FROM tracker_responses`)
	tx.Exec("DROP TABLE tracker_responses")
	tx.Exec("ALTER TABLE tracker_responses_new RENAME TO tracker_responses")
	tx.Exec("CREATE INDEX IF NOT EXISTS idx_tr_student_date ON tracker_responses(student_id, response_date)")
	tx.Exec("CREATE INDEX IF NOT EXISTS idx_tr_attendance ON tracker_responses(attendance_id)")
	tx.Commit()
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
		// Student profile fields
		"ALTER TABLE students ADD COLUMN profile_status TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE students ADD COLUMN dob TEXT",
		"ALTER TABLE students ADD COLUMN birthplace TEXT",
		"ALTER TABLE students ADD COLUMN years_in_us TEXT",
		"ALTER TABLE students ADD COLUMN first_language TEXT",
		"ALTER TABLE students ADD COLUMN previous_schools TEXT",
		"ALTER TABLE students ADD COLUMN courses_outside TEXT",
		// Parent additional contact fields
		"ALTER TABLE parents ADD COLUMN email2 TEXT",
		"ALTER TABLE parents ADD COLUMN phone2 TEXT",
		// Requires sign-off on global tracker items
		"ALTER TABLE tracker_items ADD COLUMN requires_signoff INTEGER NOT NULL DEFAULT 1",
		// Rename due_date -> end_date
		"ALTER TABLE tracker_items RENAME COLUMN due_date TO end_date",
		"ALTER TABLE student_tracker_items RENAME COLUMN due_date TO end_date",
		// Admin-controlled personal PIN (plaintext, auto-rotated daily)
		"ALTER TABLE students ADD COLUMN personal_pin TEXT",
		"ALTER TABLE students ADD COLUMN pin_generated_date TEXT",
		// Unified task_items extensions
		"ALTER TABLE task_items ADD COLUMN type TEXT NOT NULL DEFAULT 'task'",
		"ALTER TABLE task_items ADD COLUMN criteria TEXT",
		"ALTER TABLE task_items ADD COLUMN group_id TEXT",
		"ALTER TABLE task_items ADD COLUMN group_order INTEGER",
		// Late signoff support on tracker_responses
		"ALTER TABLE tracker_responses ADD COLUMN due_date TEXT",
		"ALTER TABLE tracker_responses ADD COLUMN is_late INTEGER NOT NULL DEFAULT 0",
	}
	for _, stmt := range alters {
		db.Exec(stmt) // ignore "duplicate column" errors
	}
}

// migrateToTaskItems copies data from tracker_items and student_tracker_items into
// the unified task_items table, then remaps tracker_responses.item_id references.
// Safe to call multiple times — skips if task_items already has data.
func migrateToTaskItems(db *sql.DB) {
	// Check if migration is needed
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM task_items").Scan(&count); err != nil {
		return // table doesn't exist yet
	}
	if count > 0 {
		return // already migrated
	}

	// Check if there's data to migrate
	var oldGlobal, oldStudent int
	db.QueryRow("SELECT COUNT(*) FROM tracker_items").Scan(&oldGlobal)
	db.QueryRow("SELECT COUNT(*) FROM student_tracker_items").Scan(&oldStudent)
	if oldGlobal == 0 && oldStudent == 0 {
		return
	}

	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	// Copy global items as scope=1
	tx.Exec(`INSERT INTO task_items (scope, name, notes, start_date, end_date, priority, recurrence, category,
		created_by, owner_type, requires_signoff, active, deleted, created_at, updated_at, legacy_table, legacy_id)
		SELECT 1, name, notes, start_date, end_date, priority, recurrence, category,
		created_by, 'admin', requires_signoff, active, deleted, created_at, updated_at, 'tracker_items', id
		FROM tracker_items ORDER BY id`)

	// Copy student items as scope=3
	tx.Exec(`INSERT INTO task_items (scope, student_id, name, notes, start_date, end_date, priority, recurrence, category,
		created_by, owner_type, requires_signoff, completed, completed_at, completed_by, active, deleted, created_at, updated_at, legacy_table, legacy_id)
		SELECT 3, student_id, name, notes, start_date, end_date, priority, recurrence, category,
		created_by, owner_type, requires_signoff, completed, completed_at, completed_by, active, deleted, created_at, updated_at, 'student_tracker_items', id
		FROM student_tracker_items ORDER BY id`)

	// Remap tracker_responses.item_id to new task_items IDs
	tx.Exec(`UPDATE tracker_responses SET item_id = (
		SELECT ti.id FROM task_items ti
		WHERE ti.legacy_table = 'tracker_items' AND ti.legacy_id = tracker_responses.item_id
	) WHERE item_type = 'global' AND EXISTS (
		SELECT 1 FROM task_items ti
		WHERE ti.legacy_table = 'tracker_items' AND ti.legacy_id = tracker_responses.item_id
	)`)

	tx.Exec(`UPDATE tracker_responses SET item_id = (
		SELECT ti.id FROM task_items ti
		WHERE ti.legacy_table = 'student_tracker_items' AND ti.legacy_id = tracker_responses.item_id
	) WHERE item_type = 'personal' AND EXISTS (
		SELECT 1 FROM task_items ti
		WHERE ti.legacy_table = 'student_tracker_items' AND ti.legacy_id = tracker_responses.item_id
	)`)

	// Recreate tracker_responses with updated CHECK constraint to allow 'class'
	tx.Exec(`CREATE TABLE IF NOT EXISTS tracker_responses_v2 (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id    TEXT NOT NULL,
		student_name  TEXT NOT NULL,
		item_type     TEXT NOT NULL CHECK(item_type IN ('global','personal','class')),
		item_id       INTEGER NOT NULL,
		item_name     TEXT NOT NULL,
		status        TEXT NOT NULL CHECK(status IN ('done','not_done')),
		notes         TEXT,
		response_date DATE NOT NULL DEFAULT (date('now','localtime')),
		attendance_id INTEGER,
		responded_at  DATETIME DEFAULT (datetime('now','localtime'))
	)`)
	tx.Exec(`INSERT INTO tracker_responses_v2
		SELECT id, student_id, student_name, item_type, item_id, item_name, status, notes, response_date, attendance_id, responded_at
		FROM tracker_responses`)
	tx.Exec("DROP TABLE tracker_responses")
	tx.Exec("ALTER TABLE tracker_responses_v2 RENAME TO tracker_responses")
	tx.Exec("CREATE INDEX IF NOT EXISTS idx_tr_student_date ON tracker_responses(student_id, response_date)")
	tx.Exec("CREATE INDEX IF NOT EXISTS idx_tr_attendance ON tracker_responses(attendance_id)")

	tx.Commit()
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
