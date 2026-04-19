package datastore

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"classgo/internal/models"

	"github.com/xuri/excelize/v2"
)

// ExportXLSX writes all entity data + attendance to a single XLSX workbook.
func ExportXLSX(db *sql.DB, data *EntityData) (*excelize.File, error) {
	f := excelize.NewFile()
	defer func() {
		// Don't close — caller will write and close
	}()

	// Remove default "Sheet1"
	f.DeleteSheet("Sheet1")

	writeStudentSheet(f, data.Students)
	writeParentSheet(f, data.Parents)
	writeTeacherSheet(f, data.Teachers)
	writeRoomSheet(f, data.Rooms)
	writeScheduleSheet(f, data.Schedules)

	if err := writeAttendanceSheet(f, db); err != nil {
		return nil, err
	}

	return f, nil
}

// ExportCSVDir writes all entity data to a directory of CSV files.
func ExportCSVDir(dir string, data *EntityData) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	writers := map[string]func(*csv.Writer){
		"students.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "first_name", "last_name", "grade", "school", "parent_id", "notes", "active"})
			for _, s := range data.Students {
				w.Write([]string{s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Notes, boolStr(s.Active)})
			}
		},
		"parents.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "first_name", "last_name", "email", "phone", "notes"})
			for _, p := range data.Parents {
				w.Write([]string{p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Notes})
			}
		},
		"teachers.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "first_name", "last_name", "email", "phone", "subjects", "active"})
			for _, t := range data.Teachers {
				w.Write([]string{t.ID, t.FirstName, t.LastName, t.Email, t.Phone, strings.Join(t.Subjects, ";"), boolStr(t.Active)})
			}
		},
		"rooms.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "name", "capacity", "notes"})
			for _, r := range data.Rooms {
				w.Write([]string{r.ID, r.Name, fmt.Sprint(r.Capacity), r.Notes})
			}
		},
		"schedules.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "day_of_week", "start_time", "end_time", "teacher_id", "room_id", "subject", "student_ids", "effective_from", "effective_until"})
			for _, s := range data.Schedules {
				w.Write([]string{s.ID, s.DayOfWeek, s.StartTime, s.EndTime, s.TeacherID, s.RoomID, s.Subject, strings.Join(s.StudentIDs, ";"), s.EffectiveFrom, s.EffectiveUntil})
			}
		},
	}

	for name, writeFn := range writers {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		w := csv.NewWriter(f)
		writeFn(w)
		w.Flush()
		f.Close()
	}
	return nil
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func writeStudentSheet(f *excelize.File, students []models.Student) {
	sheet := "Students"
	f.NewSheet(sheet)
	headers := []string{"id", "first_name", "last_name", "grade", "school", "parent_id", "notes", "active"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, s := range students {
		row := r + 2
		vals := []string{s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Notes, boolStr(s.Active)}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeParentSheet(f *excelize.File, parents []models.Parent) {
	sheet := "Parents"
	f.NewSheet(sheet)
	headers := []string{"id", "first_name", "last_name", "email", "phone", "notes"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, p := range parents {
		row := r + 2
		vals := []string{p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Notes}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeTeacherSheet(f *excelize.File, teachers []models.Teacher) {
	sheet := "Teachers"
	f.NewSheet(sheet)
	headers := []string{"id", "first_name", "last_name", "email", "phone", "subjects", "active"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, t := range teachers {
		row := r + 2
		vals := []string{t.ID, t.FirstName, t.LastName, t.Email, t.Phone, strings.Join(t.Subjects, ";"), boolStr(t.Active)}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeRoomSheet(f *excelize.File, rooms []models.Room) {
	sheet := "Rooms"
	f.NewSheet(sheet)
	headers := []string{"id", "name", "capacity", "notes"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, rm := range rooms {
		row := r + 2
		f.SetCellValue(sheet, cellName(1, row), rm.ID)
		f.SetCellValue(sheet, cellName(2, row), rm.Name)
		f.SetCellValue(sheet, cellName(3, row), rm.Capacity)
		f.SetCellValue(sheet, cellName(4, row), rm.Notes)
	}
}

func writeScheduleSheet(f *excelize.File, schedules []models.Schedule) {
	sheet := "Schedules"
	f.NewSheet(sheet)
	headers := []string{"id", "day_of_week", "start_time", "end_time", "teacher_id", "room_id", "subject", "student_ids", "effective_from", "effective_until"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, s := range schedules {
		row := r + 2
		vals := []string{s.ID, s.DayOfWeek, s.StartTime, s.EndTime, s.TeacherID, s.RoomID, s.Subject, strings.Join(s.StudentIDs, ";"), s.EffectiveFrom, s.EffectiveUntil}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeAttendanceSheet(f *excelize.File, db *sql.DB) error {
	sheet := "Attendance"
	f.NewSheet(sheet)
	headers := []string{"ID", "Student Name", "Device Type", "Sign In", "Sign Out", "Duration"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	rows, err := db.Query("SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance ORDER BY sign_in_time DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	r := 2
	for rows.Next() {
		var id int
		var studentName, deviceType, signIn string
		var signOut sql.NullString
		if err := rows.Scan(&id, &studentName, &deviceType, &signIn, &signOut); err != nil {
			continue
		}
		inTime, _ := models.ParseTimestamp(signIn)
		signInFmt := inTime.Format("2006-01-02 3:04 PM")
		signOutFmt := ""
		durationStr := ""
		if signOut.Valid {
			outTime, _ := models.ParseTimestamp(signOut.String)
			signOutFmt = outTime.Format("2006-01-02 3:04 PM")
			durationStr = models.FormatDuration(outTime.Sub(inTime))
		}

		f.SetCellValue(sheet, cellName(1, r), id)
		f.SetCellValue(sheet, cellName(2, r), studentName)
		f.SetCellValue(sheet, cellName(3, r), deviceType)
		f.SetCellValue(sheet, cellName(4, r), signInFmt)
		f.SetCellValue(sheet, cellName(5, r), signOutFmt)
		f.SetCellValue(sheet, cellName(6, r), durationStr)
		r++
	}
	return nil
}

// ReadFromDB reads all entity data back from SQLite index tables.
func ReadFromDB(db *sql.DB) (*EntityData, error) {
	data := &EntityData{}
	var err error

	data.Students, err = queryStudents(db)
	if err != nil {
		return nil, err
	}
	data.Parents, err = queryParents(db)
	if err != nil {
		return nil, err
	}
	data.Teachers, err = queryTeachers(db)
	if err != nil {
		return nil, err
	}
	data.Rooms, err = queryRooms(db)
	if err != nil {
		return nil, err
	}
	data.Schedules, err = querySchedules(db)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func queryStudents(db *sql.DB) ([]models.Student, error) {
	rows, err := db.Query("SELECT id, first_name, last_name, grade, school, parent_id, notes, active FROM students ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Student
	for rows.Next() {
		var s models.Student
		var active int
		var grade, school, parentID, notes sql.NullString
		if err := rows.Scan(&s.ID, &s.FirstName, &s.LastName, &grade, &school, &parentID, &notes, &active); err != nil {
			return nil, err
		}
		s.Grade = grade.String
		s.School = school.String
		s.ParentID = parentID.String
		s.Notes = notes.String
		s.Active = active == 1
		result = append(result, s)
	}
	return result, rows.Err()
}

func queryParents(db *sql.DB) ([]models.Parent, error) {
	rows, err := db.Query("SELECT id, first_name, last_name, email, phone, notes FROM parents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Parent
	for rows.Next() {
		var p models.Parent
		var email, phone, notes sql.NullString
		if err := rows.Scan(&p.ID, &p.FirstName, &p.LastName, &email, &phone, &notes); err != nil {
			return nil, err
		}
		p.Email = email.String
		p.Phone = phone.String
		p.Notes = notes.String
		result = append(result, p)
	}
	return result, rows.Err()
}

func queryTeachers(db *sql.DB) ([]models.Teacher, error) {
	rows, err := db.Query("SELECT id, first_name, last_name, email, phone, subjects, active FROM teachers ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Teacher
	for rows.Next() {
		var t models.Teacher
		var email, phone, subjects, notes sql.NullString
		var active int
		if err := rows.Scan(&t.ID, &t.FirstName, &t.LastName, &email, &phone, &subjects, &active); err != nil {
			return nil, err
		}
		t.Email = email.String
		t.Phone = phone.String
		t.Subjects = splitSemicolon(subjects.String)
		_ = notes
		t.Active = active == 1
		result = append(result, t)
	}
	return result, rows.Err()
}

func queryRooms(db *sql.DB) ([]models.Room, error) {
	rows, err := db.Query("SELECT id, name, capacity, notes FROM rooms ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Room
	for rows.Next() {
		var r models.Room
		var capacity sql.NullInt64
		var notes sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &capacity, &notes); err != nil {
			return nil, err
		}
		r.Capacity = int(capacity.Int64)
		r.Notes = notes.String
		result = append(result, r)
	}
	return result, rows.Err()
}

func querySchedules(db *sql.DB) ([]models.Schedule, error) {
	rows, err := db.Query("SELECT id, day_of_week, start_time, end_time, teacher_id, room_id, subject, student_ids, effective_from, effective_until FROM schedules ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var teacherID, roomID, subject, studentIDs, effectiveFrom, effectiveUntil sql.NullString
		if err := rows.Scan(&s.ID, &s.DayOfWeek, &s.StartTime, &s.EndTime, &teacherID, &roomID, &subject, &studentIDs, &effectiveFrom, &effectiveUntil); err != nil {
			return nil, err
		}
		s.TeacherID = teacherID.String
		s.RoomID = roomID.String
		s.Subject = subject.String
		s.StudentIDs = splitSemicolon(studentIDs.String)
		s.EffectiveFrom = effectiveFrom.String
		s.EffectiveUntil = effectiveUntil.String
		result = append(result, s)
	}
	return result, rows.Err()
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

