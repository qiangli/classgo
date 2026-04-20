package datastore

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"classgo/internal/models"

	"github.com/xuri/excelize/v2"
)

// EntityData holds all parsed entity data from spreadsheet files.
type EntityData struct {
	Students  []models.Student
	Parents   []models.Parent
	Teachers  []models.Teacher
	Rooms     []models.Room
	Schedules []models.Schedule
}

// ReadAll reads entity data from the data directory.
// It checks for tutoros.xlsx first; if not found, falls back to csv/ folder.
func ReadAll(dataDir string) (*EntityData, error) {
	xlsxPath := filepath.Join(dataDir, "tutoros.xlsx")
	if _, err := os.Stat(xlsxPath); err == nil {
		return readXLSX(xlsxPath)
	}

	csvDir := filepath.Join(dataDir, "csv")
	if info, err := os.Stat(csvDir); err == nil && info.IsDir() {
		return readCSVDir(csvDir)
	}

	// Fall back to CSV files directly in the data directory
	if _, err := os.Stat(filepath.Join(dataDir, "students.csv")); err == nil {
		return readCSVDir(dataDir)
	}

	return &EntityData{}, nil
}

func readXLSX(path string) (*EntityData, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	data := &EntityData{}

	if rows, err := getSheetRows(f, "Parents"); err == nil {
		data.Parents = parseParentRows(rows)
	}
	if rows, err := getSheetRows(f, "Students"); err == nil {
		data.Students = parseStudentRows(rows)
	}
	if rows, err := getSheetRows(f, "Teachers"); err == nil {
		data.Teachers = parseTeacherRows(rows)
	}
	if rows, err := getSheetRows(f, "Rooms"); err == nil {
		data.Rooms = parseRoomRows(rows)
	}
	if rows, err := getSheetRows(f, "Schedules"); err == nil {
		data.Schedules = parseScheduleRows(rows)
	}

	return data, nil
}

func getSheetRows(f *excelize.File, sheet string) ([][]string, error) {
	idx, err := f.GetSheetIndex(sheet)
	if err != nil || idx < 0 {
		return nil, fmt.Errorf("sheet %q not found", sheet)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func readCSVDir(dir string) (*EntityData, error) {
	data := &EntityData{}

	if rows, err := readCSVFile(filepath.Join(dir, "parents.csv")); err == nil {
		data.Parents = parseParentRows(rows)
	}
	if rows, err := readCSVFile(filepath.Join(dir, "students.csv")); err == nil {
		data.Students = parseStudentRows(rows)
	}
	if rows, err := readCSVFile(filepath.Join(dir, "teachers.csv")); err == nil {
		data.Teachers = parseTeacherRows(rows)
	}
	if rows, err := readCSVFile(filepath.Join(dir, "rooms.csv")); err == nil {
		data.Rooms = parseRoomRows(rows)
	}
	if rows, err := readCSVFile(filepath.Join(dir, "schedules.csv")); err == nil {
		data.Schedules = parseScheduleRows(rows)
	}

	return data, nil
}

func readCSVFile(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return csv.NewReader(f).ReadAll()
}

// rowMap maps header names to values for a single row, providing safe column access.
type rowMap map[string]string

func makeRowMaps(rows [][]string) []rowMap {
	if len(rows) < 2 {
		return nil
	}
	headers := rows[0]
	var result []rowMap
	for _, row := range rows[1:] {
		m := make(rowMap)
		for i, h := range headers {
			h = strings.TrimSpace(strings.ToLower(h))
			if i < len(row) {
				m[h] = strings.TrimSpace(row[i])
			}
		}
		if m["id"] == "" {
			continue
		}
		result = append(result, m)
	}
	return result
}

func parseStudentRows(rows [][]string) []models.Student {
	var students []models.Student
	for _, m := range makeRowMaps(rows) {
		students = append(students, models.Student{
			ID:              m["id"],
			FirstName:       m["first_name"],
			LastName:        m["last_name"],
			Grade:           m["grade"],
			School:          m["school"],
			ParentID:        parseRef(m["parent_id"]),
			Email:           m["email"],
			Phone:           m["phone"],
			Address:         m["address"],
			Notes:           m["notes"],
			DOB:             m["dob"],
			Birthplace:      m["birthplace"],
			YearsInUS:       m["years_in_us"],
			FirstLanguage:   m["first_language"],
			PreviousSchools: m["previous_schools"],
			CoursesOutside:  m["courses_outside"],
			Active:          parseBool(m["active"]),
			Deleted:         parseBoolDefault(m["deleted"], false),
		})
	}
	return students
}

func parseParentRows(rows [][]string) []models.Parent {
	var parents []models.Parent
	for _, m := range makeRowMaps(rows) {
		parents = append(parents, models.Parent{
			ID:        m["id"],
			FirstName: m["first_name"],
			LastName:  m["last_name"],
			Email:     m["email"],
			Phone:     m["phone"],
			Email2:    m["email2"],
			Phone2:    m["phone2"],
			Address:   m["address"],
			Notes:     m["notes"],
			Deleted:   parseBoolDefault(m["deleted"], false),
		})
	}
	return parents
}

func parseTeacherRows(rows [][]string) []models.Teacher {
	var teachers []models.Teacher
	for _, m := range makeRowMaps(rows) {
		teachers = append(teachers, models.Teacher{
			ID:        m["id"],
			FirstName: m["first_name"],
			LastName:  m["last_name"],
			Email:     m["email"],
			Phone:     m["phone"],
			Address:   m["address"],
			Subjects:  splitSemicolon(m["subjects"]),
			Active:    parseBool(m["active"]),
			Deleted:   parseBoolDefault(m["deleted"], false),
		})
	}
	return teachers
}

func parseRoomRows(rows [][]string) []models.Room {
	var rooms []models.Room
	for _, m := range makeRowMaps(rows) {
		cap, _ := strconv.Atoi(m["capacity"])
		rooms = append(rooms, models.Room{
			ID:       m["id"],
			Name:     m["name"],
			Capacity: cap,
			Notes:    m["notes"],
			Deleted:  parseBoolDefault(m["deleted"], false),
		})
	}
	return rooms
}

func parseScheduleRows(rows [][]string) []models.Schedule {
	var schedules []models.Schedule
	for _, m := range makeRowMaps(rows) {
		schedules = append(schedules, models.Schedule{
			ID:             m["id"],
			DayOfWeek:      m["day_of_week"],
			StartTime:      m["start_time"],
			EndTime:        m["end_time"],
			TeacherID:      parseRef(m["teacher_id"]),
			RoomID:         parseRef(m["room_id"]),
			Subject:        m["subject"],
			StudentIDs:     parseRefList(m["student_ids"]),
			EffectiveFrom:  m["effective_from"],
			EffectiveUntil: m["effective_until"],
			Deleted:        parseBoolDefault(m["deleted"], false),
		})
	}
	return schedules
}

func splitSemicolon(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "" || s == "yes" || s == "true" || s == "1"
}

func parseBoolDefault(s string, defaultVal bool) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return defaultVal
	}
	return s == "yes" || s == "true" || s == "1"
}

// parseRefList parses a semicolon-separated list of cross-reference strings.
func parseRefList(s string) []string {
	parts := splitSemicolon(s)
	for i, p := range parts {
		parts[i] = parseRef(p)
	}
	return parts
}

// parseRef extracts an ID from a cross-reference string.
// Supported formats:
//   - "Name/ID" (e.g., "Wei Wang/P001") → returns "P001"
//   - "ID" (e.g., "P001") → returns "P001"
//   - "Name" (e.g., "Wei Wang") → returns "Wei Wang" as-is (caller may resolve)
func parseRef(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return strings.TrimSpace(s[idx+1:])
	}
	return s
}
