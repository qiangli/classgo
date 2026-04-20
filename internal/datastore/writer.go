package datastore

import (
	"classgo/internal/models"
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
			w.Write([]string{"id", "first_name", "last_name", "grade", "school", "parent_id", "email", "phone", "address", "notes",
				"dob", "birthplace", "years_in_us", "first_language", "previous_schools", "courses_outside", "active"})
			for _, s := range data.Students {
				w.Write([]string{s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Email, s.Phone, s.Address, s.Notes,
					s.DOB, s.Birthplace, s.YearsInUS, s.FirstLanguage, s.PreviousSchools, s.CoursesOutside, boolStr(s.Active)})
			}
		},
		"parents.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "first_name", "last_name", "email", "phone", "email2", "phone2", "address", "notes"})
			for _, p := range data.Parents {
				w.Write([]string{p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Email2, p.Phone2, p.Address, p.Notes})
			}
		},
		"teachers.csv": func(w *csv.Writer) {
			w.Write([]string{"id", "first_name", "last_name", "email", "phone", "address", "subjects", "active"})
			for _, t := range data.Teachers {
				w.Write([]string{t.ID, t.FirstName, t.LastName, t.Email, t.Phone, t.Address, strings.Join(t.Subjects, ";"), boolStr(t.Active)})
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
	headers := []string{"id", "first_name", "last_name", "grade", "school", "parent_id", "email", "phone", "address", "notes",
		"dob", "birthplace", "years_in_us", "first_language", "previous_schools", "courses_outside", "active"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, s := range students {
		row := r + 2
		vals := []string{s.ID, s.FirstName, s.LastName, s.Grade, s.School, s.ParentID, s.Email, s.Phone, s.Address, s.Notes,
			s.DOB, s.Birthplace, s.YearsInUS, s.FirstLanguage, s.PreviousSchools, s.CoursesOutside, boolStr(s.Active)}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeParentSheet(f *excelize.File, parents []models.Parent) {
	sheet := "Parents"
	f.NewSheet(sheet)
	headers := []string{"id", "first_name", "last_name", "email", "phone", "email2", "phone2", "address", "notes"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, p := range parents {
		row := r + 2
		vals := []string{p.ID, p.FirstName, p.LastName, p.Email, p.Phone, p.Email2, p.Phone2, p.Address, p.Notes}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeTeacherSheet(f *excelize.File, teachers []models.Teacher) {
	sheet := "Teachers"
	f.NewSheet(sheet)
	headers := []string{"id", "first_name", "last_name", "email", "phone", "address", "subjects", "active"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, t := range teachers {
		row := r + 2
		vals := []string{t.ID, t.FirstName, t.LastName, t.Email, t.Phone, t.Address, strings.Join(t.Subjects, ";"), boolStr(t.Active)}
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
	headers := []string{"ID", "Student Name", "Device Type", "Check In", "Check Out", "Duration"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	rows, err := db.Query("SELECT id, student_name, device_type, check_in_time, check_out_time FROM attendance ORDER BY check_in_time DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	r := 2
	for rows.Next() {
		var id int
		var studentName, deviceType, checkIn string
		var checkOut sql.NullString
		if err := rows.Scan(&id, &studentName, &deviceType, &checkIn, &checkOut); err != nil {
			continue
		}
		inTime, _ := models.ParseTimestamp(checkIn)
		checkInFmt := inTime.Format("2006-01-02 3:04 PM")
		checkOutFmt := ""
		durationStr := ""
		if checkOut.Valid {
			outTime, _ := models.ParseTimestamp(checkOut.String)
			checkOutFmt = outTime.Format("2006-01-02 3:04 PM")
			durationStr = models.FormatDuration(outTime.Sub(inTime))
		}

		f.SetCellValue(sheet, cellName(1, r), id)
		f.SetCellValue(sheet, cellName(2, r), studentName)
		f.SetCellValue(sheet, cellName(3, r), deviceType)
		f.SetCellValue(sheet, cellName(4, r), checkInFmt)
		f.SetCellValue(sheet, cellName(5, r), checkOutFmt)
		f.SetCellValue(sheet, cellName(6, r), durationStr)
		r++
	}
	return nil
}

// ReadFromDB reads all non-deleted entity data back from SQLite index tables.
func ReadFromDB(db *sql.DB) (*EntityData, error) {
	return readFromDB(db, false)
}

// ReadFromDBAll reads all entity data including soft-deleted records.
func ReadFromDBAll(db *sql.DB) (*EntityData, error) {
	return readFromDB(db, true)
}

func readFromDB(db *sql.DB, includeDeleted bool) (*EntityData, error) {
	data := &EntityData{}
	var err error

	data.Students, err = queryStudents(db, includeDeleted)
	if err != nil {
		return nil, err
	}
	data.Parents, err = queryParents(db, includeDeleted)
	if err != nil {
		return nil, err
	}
	data.Teachers, err = queryTeachers(db, includeDeleted)
	if err != nil {
		return nil, err
	}
	data.Rooms, err = queryRooms(db, includeDeleted)
	if err != nil {
		return nil, err
	}
	data.Schedules, err = querySchedules(db, includeDeleted)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func queryStudents(db *sql.DB, includeDeleted bool) ([]models.Student, error) {
	q := `SELECT id, first_name, last_name, grade, school, parent_id, email, phone, address, notes,
	      COALESCE(dob,''), COALESCE(birthplace,''), COALESCE(years_in_us,''), COALESCE(first_language,''),
	      COALESCE(previous_schools,''), COALESCE(courses_outside,''), COALESCE(profile_status,''),
	      active, deleted, COALESCE(require_pin, 0) FROM students`
	if !includeDeleted {
		q += " WHERE deleted = 0"
	}
	q += " ORDER BY id"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Student
	for rows.Next() {
		var s models.Student
		var active, deleted, requirePIN int
		var grade, school, parentID, email, phone, address, notes sql.NullString
		if err := rows.Scan(&s.ID, &s.FirstName, &s.LastName, &grade, &school, &parentID, &email, &phone, &address, &notes,
			&s.DOB, &s.Birthplace, &s.YearsInUS, &s.FirstLanguage, &s.PreviousSchools, &s.CoursesOutside, &s.ProfileStatus,
			&active, &deleted, &requirePIN); err != nil {
			return nil, err
		}
		s.Grade = grade.String
		s.School = school.String
		s.ParentID = parentID.String
		s.Email = email.String
		s.Phone = phone.String
		s.Address = address.String
		s.Notes = notes.String
		s.Active = active == 1
		s.Deleted = deleted == 1
		s.RequirePIN = requirePIN == 1
		result = append(result, s)
	}
	return result, rows.Err()
}

func queryParents(db *sql.DB, includeDeleted bool) ([]models.Parent, error) {
	q := "SELECT id, first_name, last_name, email, phone, COALESCE(email2,''), COALESCE(phone2,''), address, notes, deleted FROM parents"
	if !includeDeleted {
		q += " WHERE deleted = 0"
	}
	q += " ORDER BY id"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Parent
	for rows.Next() {
		var p models.Parent
		var email, phone, address, notes sql.NullString
		var deleted int
		if err := rows.Scan(&p.ID, &p.FirstName, &p.LastName, &email, &phone, &p.Email2, &p.Phone2, &address, &notes, &deleted); err != nil {
			return nil, err
		}
		p.Email = email.String
		p.Phone = phone.String
		p.Address = address.String
		p.Notes = notes.String
		p.Deleted = deleted == 1
		result = append(result, p)
	}
	return result, rows.Err()
}

func queryTeachers(db *sql.DB, includeDeleted bool) ([]models.Teacher, error) {
	q := "SELECT id, first_name, last_name, email, phone, address, subjects, active, deleted FROM teachers"
	if !includeDeleted {
		q += " WHERE deleted = 0"
	}
	q += " ORDER BY id"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Teacher
	for rows.Next() {
		var t models.Teacher
		var email, phone, address, subjects sql.NullString
		var active, deleted int
		if err := rows.Scan(&t.ID, &t.FirstName, &t.LastName, &email, &phone, &address, &subjects, &active, &deleted); err != nil {
			return nil, err
		}
		t.Email = email.String
		t.Phone = phone.String
		t.Address = address.String
		t.Subjects = splitSemicolon(subjects.String)
		t.Active = active == 1
		t.Deleted = deleted == 1
		result = append(result, t)
	}
	return result, rows.Err()
}

func queryRooms(db *sql.DB, includeDeleted bool) ([]models.Room, error) {
	q := "SELECT id, name, capacity, notes, deleted FROM rooms"
	if !includeDeleted {
		q += " WHERE deleted = 0"
	}
	q += " ORDER BY id"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Room
	for rows.Next() {
		var r models.Room
		var capacity sql.NullInt64
		var notes sql.NullString
		var deleted int
		if err := rows.Scan(&r.ID, &r.Name, &capacity, &notes, &deleted); err != nil {
			return nil, err
		}
		r.Capacity = int(capacity.Int64)
		r.Notes = notes.String
		r.Deleted = deleted == 1
		result = append(result, r)
	}
	return result, rows.Err()
}

func querySchedules(db *sql.DB, includeDeleted bool) ([]models.Schedule, error) {
	q := "SELECT id, day_of_week, start_time, end_time, teacher_id, room_id, subject, student_ids, effective_from, effective_until, deleted FROM schedules"
	if !includeDeleted {
		q += " WHERE deleted = 0"
	}
	q += " ORDER BY id"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var teacherID, roomID, subject, studentIDs, effectiveFrom, effectiveUntil sql.NullString
		var deleted int
		if err := rows.Scan(&s.ID, &s.DayOfWeek, &s.StartTime, &s.EndTime, &teacherID, &roomID, &subject, &studentIDs, &effectiveFrom, &effectiveUntil, &deleted); err != nil {
			return nil, err
		}
		s.TeacherID = teacherID.String
		s.RoomID = roomID.String
		s.Subject = subject.String
		s.StudentIDs = splitSemicolon(studentIDs.String)
		s.EffectiveFrom = effectiveFrom.String
		s.EffectiveUntil = effectiveUntil.String
		s.Deleted = deleted == 1
		result = append(result, s)
	}
	return result, rows.Err()
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
