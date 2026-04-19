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
			ID:        m["id"],
			FirstName: m["first_name"],
			LastName:  m["last_name"],
			Grade:     m["grade"],
			School:    m["school"],
			ParentID:  m["parent_id"],
			Notes:     m["notes"],
			Active:    parseBool(m["active"]),
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
			Notes:     m["notes"],
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
			Subjects:  splitSemicolon(m["subjects"]),
			Active:    parseBool(m["active"]),
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
			TeacherID:      m["teacher_id"],
			RoomID:         m["room_id"],
			Subject:        m["subject"],
			StudentIDs:     splitSemicolon(m["student_ids"]),
			EffectiveFrom:  m["effective_from"],
			EffectiveUntil: m["effective_until"],
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
