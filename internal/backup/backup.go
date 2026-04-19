package backup

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Run creates a full backup of all tables in the given databases as a CSV ZIP file.
// The ZIP is saved to backupDir with a timestamped filename.
// Returns the path to the created backup file.
func Run(backupDir string, databases map[string]*sql.DB) (string, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_150405")
	filename := fmt.Sprintf("backup-%s.zip", timestamp)
	zipPath := filepath.Join(backupDir, filename)

	f, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	totalTables := 0
	totalRows := 0

	for dbName, db := range databases {
		tables, err := listTables(db)
		if err != nil {
			log.Printf("Backup: failed to list tables for %s: %v", dbName, err)
			continue
		}

		for _, table := range tables {
			rows, err := exportTable(zw, db, dbName, table)
			if err != nil {
				log.Printf("Backup: failed to export %s.%s: %v", dbName, table, err)
				continue
			}
			totalTables++
			totalRows += rows
		}
	}

	log.Printf("Backup: %s (%d tables, %d rows)", filename, totalTables, totalRows)
	return zipPath, nil
}

// RunOnShutdown creates a backup tagged as a shutdown backup.
func RunOnShutdown(backupDir string, databases map[string]*sql.DB) {
	log.Println("Backup: creating shutdown backup...")
	path, err := Run(backupDir, databases)
	if err != nil {
		log.Printf("Backup: shutdown backup failed: %v", err)
		return
	}
	log.Printf("Backup: shutdown backup saved to %s", path)
}

// listTables returns all user table names in the database.
func listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// exportTable writes a single table as a CSV entry in the ZIP.
// Returns the number of rows written.
func exportTable(zw *zip.Writer, db *sql.DB, dbName, tableName string) (int, error) {
	// Query all rows
	rows, err := db.Query("SELECT * FROM " + quoteName(tableName))
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", tableName, err)
	}
	defer rows.Close()

	// Get column names
	cols, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("columns %s: %w", tableName, err)
	}

	// Create CSV file in ZIP: dbname/tablename.csv
	csvName := fmt.Sprintf("%s/%s.csv", dbName, tableName)
	w, err := zw.Create(csvName)
	if err != nil {
		return 0, fmt.Errorf("create zip entry: %w", err)
	}

	cw := csv.NewWriter(w)

	// Write header
	if err := cw.Write(cols); err != nil {
		return 0, err
	}

	// Write rows
	count := 0
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		record := make([]string, len(cols))
		for i, v := range values {
			record[i] = formatValue(v)
		}
		if err := cw.Write(record); err != nil {
			return count, err
		}
		count++
	}

	cw.Flush()
	return count, cw.Error()
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Restore restores tables from the latest backup ZIP file in backupDir.
// It only restores tables that are empty in the target database.
// Returns the backup file used, the number of tables restored, and any error.
func Restore(backupDir string, databases map[string]*sql.DB) (string, int, error) {
	latest, err := findLatestBackup(backupDir)
	if err != nil || latest == "" {
		return "", 0, nil // no backups found — not an error
	}

	f, err := zip.OpenReader(latest)
	if err != nil {
		return latest, 0, fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	restored := 0
	var restoreErr error

	for _, entry := range f.File {
		// Parse "dbname/tablename.csv" from ZIP entry name
		parts := strings.SplitN(entry.Name, "/", 2)
		if len(parts) != 2 || !strings.HasSuffix(parts[1], ".csv") {
			continue
		}
		dbName := parts[0]
		tableName := strings.TrimSuffix(parts[1], ".csv")

		db, ok := databases[dbName]
		if !ok {
			continue
		}

		// Only restore if the table is empty
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + quoteName(tableName)).Scan(&count)
		if err != nil || count > 0 {
			continue // table doesn't exist or already has data
		}

		rc, err := entry.Open()
		if err != nil {
			restoreErr = fmt.Errorf("open %s: %w", entry.Name, err)
			continue
		}

		reader := csv.NewReader(rc)
		records, err := reader.ReadAll()
		rc.Close()
		if err != nil || len(records) < 2 {
			continue // empty or header-only CSV
		}

		cols := records[0]
		placeholders := strings.Repeat("?,", len(cols))
		placeholders = placeholders[:len(placeholders)-1]
		insertSQL := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)",
			quoteName(tableName),
			strings.Join(quoteNames(cols), ","),
			placeholders,
		)

		tx, err := db.Begin()
		if err != nil {
			restoreErr = fmt.Errorf("begin tx for %s.%s: %w", dbName, tableName, err)
			continue
		}

		stmt, err := tx.Prepare(insertSQL)
		if err != nil {
			tx.Rollback()
			continue // column mismatch — schema changed, skip silently
		}

		rowCount := 0
		for _, record := range records[1:] {
			args := make([]any, len(record))
			for i, v := range record {
				if v == "" {
					args[i] = nil
				} else {
					args[i] = v
				}
			}
			if _, err := stmt.Exec(args...); err != nil {
				continue // skip bad rows
			}
			rowCount++
		}
		stmt.Close()

		if err := tx.Commit(); err != nil {
			restoreErr = fmt.Errorf("commit %s.%s: %w", dbName, tableName, err)
			continue
		}

		if rowCount > 0 {
			log.Printf("Backup: restored %d rows into %s.%s", rowCount, dbName, tableName)
			restored++
		}
	}

	return latest, restored, restoreErr
}

// findLatestBackup returns the path to the most recent backup-*.zip file.
func findLatestBackup(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var latest string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "backup-") && strings.HasSuffix(name, ".zip") {
			path := filepath.Join(dir, name)
			if latest == "" || name > filepath.Base(latest) {
				latest = path
			}
		}
	}
	return latest, nil
}

func quoteNames(names []string) []string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = quoteName(n)
	}
	return quoted
}

// quoteName wraps a table name in double quotes to handle reserved words.
func quoteName(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
