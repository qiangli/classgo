package reports

import (
	"database/sql"
	"fmt"
	"time"

	"classgo/internal/database"
)

// WeeklyHoursActivity generates the T1 report for a teacher.
func WeeklyHoursActivity(db *sql.DB, dr DateRange, teacherID string) (*TeacherWeeklyHoursReport, error) {
	report := &TeacherWeeklyHoursReport{
		DateRange: dr,
		TeacherID: teacherID,
	}

	// Lookup teacher name
	db.QueryRow("SELECT first_name || ' ' || last_name FROM teachers WHERE id = ?", teacherID).Scan(&report.TeacherName)

	// Get this teacher's schedules
	rows, err := db.Query(
		`SELECT id, day_of_week, start_time, end_time, COALESCE(subject,''), COALESCE(room_id,''), COALESCE(student_ids,'')
		 FROM schedules WHERE teacher_id = ? AND deleted = 0
		 ORDER BY CASE day_of_week WHEN 'Monday' THEN 1 WHEN 'Tuesday' THEN 2 WHEN 'Wednesday' THEN 3
		 WHEN 'Thursday' THEN 4 WHEN 'Friday' THEN 5 WHEN 'Saturday' THEN 6 WHEN 'Sunday' THEN 7 END,
		 start_time`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Days in the date range for matching schedules
	fromT, _ := time.ParseInLocation("2006-01-02", dr.From, time.Local)
	toT, _ := time.ParseInLocation("2006-01-02", dr.To, time.Local)
	daysInRange := map[string]bool{}
	for d := fromT; !d.After(toT); d = d.AddDate(0, 0, 1) {
		daysInRange[d.Weekday().String()] = true
	}

	uniqueStudents := map[string]bool{}

	for rows.Next() {
		var c ClassSummaryRow
		var roomID, sids string
		rows.Scan(&c.ScheduleID, &c.DayOfWeek, &c.StartTime, &c.EndTime, &c.Subject, &roomID, &sids)

		// Only count classes whose day falls within the date range
		if !daysInRange[c.DayOfWeek] {
			continue
		}

		// Room name lookup
		if roomID != "" {
			db.QueryRow("SELECT COALESCE(name,'') FROM rooms WHERE id = ?", roomID).Scan(&c.RoomName)
		}

		enrolledIDs := splitIDs(sids)
		c.Enrolled = len(enrolledIDs)
		hours := parseScheduleHours(c.StartTime, c.EndTime)
		report.TotalHours += hours
		report.ClassCount++

		// Attendance for this schedule this week
		attendedSet := map[string]bool{}
		aRows, _ := db.Query(
			`SELECT DISTINCT am.student_id FROM attendance_meta am
			 JOIN attendance a ON a.id = am.attendance_id
			 WHERE am.schedule_id = ? AND date(a.check_in_time) >= ? AND date(a.check_in_time) <= ?`,
			c.ScheduleID, dr.From, dr.To)
		if aRows != nil {
			for aRows.Next() {
				var sid string
				aRows.Scan(&sid)
				attendedSet[sid] = true
				uniqueStudents[sid] = true
			}
			aRows.Close()
		}
		c.Attended = len(attendedSet)

		// Absent list
		for _, sid := range enrolledIDs {
			uniqueStudents[sid] = true
			if !attendedSet[sid] {
				var name string
				db.QueryRow("SELECT first_name || ' ' || last_name FROM students WHERE id = ?", sid).Scan(&name)
				if name == "" {
					name = sid
				}
				c.Absent = append(c.Absent, name)
			}
		}

		report.Classes = append(report.Classes, c)
	}

	report.UniqueStudents = len(uniqueStudents)
	return report, nil
}

// BiweeklyStudentSummary generates the T2 report.
func BiweeklyStudentSummary(db *sql.DB, dr DateRange, teacherID string) (*TeacherBiweeklyReport, error) {
	report := &TeacherBiweeklyReport{
		DateRange: dr,
		TeacherID: teacherID,
	}
	db.QueryRow("SELECT first_name || ' ' || last_name FROM teachers WHERE id = ?", teacherID).Scan(&report.TeacherName)

	studentIDs, err := database.GetTeacherStudentIDs(db, teacherID)
	if err != nil {
		return nil, err
	}

	// Get progress stats for these students
	progressMap := map[string]float64{}
	stats, _ := database.GetProgressStats(db, studentIDs, dr.From, dr.To)
	for _, s := range stats {
		progressMap[s.StudentID] = s.Completion
	}

	// Compare with previous period for trend
	fromT, _ := time.ParseInLocation("2006-01-02", dr.From, time.Local)
	prevDays := int(mustParseDate(dr.To).Sub(fromT).Hours() / 24)
	prevFrom := fromT.AddDate(0, 0, -prevDays).Format("2006-01-02")
	prevTo := fromT.AddDate(0, 0, -1).Format("2006-01-02")
	prevProgressMap := map[string]float64{}
	prevStats, _ := database.GetProgressStats(db, studentIDs, prevFrom, prevTo)
	for _, s := range prevStats {
		prevProgressMap[s.StudentID] = s.Completion
	}

	for _, sid := range studentIDs {
		var fn, ln string
		db.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", sid).Scan(&fn, &ln)

		sp := TeacherStudentProgress{
			StudentID:   sid,
			StudentName: fn + " " + ln,
		}

		// Attendance
		db.QueryRow(
			`SELECT COUNT(DISTINCT date(check_in_time)),
			        COALESCE(SUM(CASE WHEN check_out_time IS NOT NULL
			            THEN (julianday(check_out_time) - julianday(check_in_time)) * 24
			            ELSE 0 END), 0)
			 FROM attendance WHERE student_id = ? AND date(check_in_time) >= ? AND date(check_in_time) <= ?`,
			sid, dr.From, dr.To).Scan(&sp.DaysAttended, &sp.TotalHours)

		sp.CompletionPct = progressMap[sid]

		// Trend
		prev := prevProgressMap[sid]
		if sp.CompletionPct > prev+5 {
			sp.Trend = "improving"
		} else if sp.CompletionPct < prev-5 {
			sp.Trend = "declining"
		} else {
			sp.Trend = "flat"
		}

		if sp.CompletionPct < 50 {
			sp.AtRisk = true
			report.AtRiskCount++
		}

		// Incomplete items count
		db.QueryRow(
			`SELECT COUNT(*) FROM tracker_responses
			 WHERE student_id = ? AND status = 'not_done'
			 AND response_date >= ? AND response_date <= ?`,
			sid, dr.From, dr.To).Scan(&sp.IncompleteItems)

		report.Students = append(report.Students, sp)
	}

	// Sort by completion ascending (lowest first)
	sortStudentProgress(report.Students)

	return report, nil
}

// MonthlyTeachingSummary generates the T3 report.
func MonthlyTeachingSummary(db *sql.DB, dr DateRange, teacherID string) (*TeacherMonthlyReport, error) {
	report := &TeacherMonthlyReport{
		DateRange: dr,
		TeacherID: teacherID,
	}
	db.QueryRow("SELECT first_name || ' ' || last_name FROM teachers WHERE id = ?", teacherID).Scan(&report.TeacherName)

	// Total hours from schedules
	schedRows, err := db.Query(
		"SELECT start_time, end_time FROM schedules WHERE teacher_id = ? AND deleted = 0", teacherID)
	if err != nil {
		return nil, err
	}
	defer schedRows.Close()

	// Count weeks in range for total hours
	fromT, _ := time.ParseInLocation("2006-01-02", dr.From, time.Local)
	toT, _ := time.ParseInLocation("2006-01-02", dr.To, time.Local)
	weeksInRange := toT.Sub(fromT).Hours() / 24 / 7
	if weeksInRange < 1 {
		weeksInRange = 1
	}

	var weeklyHours float64
	for schedRows.Next() {
		var st, et string
		schedRows.Scan(&st, &et)
		weeklyHours += parseScheduleHours(st, et)
	}
	report.TotalHours = weeklyHours * weeksInRange

	// Weekly breakdown
	weekNum := 1
	for wStart := fromT; !wStart.After(toT); {
		wEnd := wStart.AddDate(0, 0, 6)
		if wEnd.After(toT) {
			wEnd = toT
		}
		report.WeeklyHours = append(report.WeeklyHours, WeekHoursRow{
			WeekLabel: "Week " + itoa(weekNum),
			Hours:     weeklyHours,
		})
		wStart = wEnd.AddDate(0, 0, 1)
		weekNum++
	}

	// Per-student detail (reuse biweekly logic)
	studentIDs, _ := database.GetTeacherStudentIDs(db, teacherID)
	progressMap := map[string]float64{}
	stats, _ := database.GetProgressStats(db, studentIDs, dr.From, dr.To)
	for _, s := range stats {
		progressMap[s.StudentID] = s.Completion
	}

	var totalAttRate, totalCompRate float64
	for _, sid := range studentIDs {
		var fn, ln string
		db.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", sid).Scan(&fn, &ln)

		sp := TeacherStudentProgress{
			StudentID:     sid,
			StudentName:   fn + " " + ln,
			CompletionPct: progressMap[sid],
		}

		db.QueryRow(
			`SELECT COUNT(DISTINCT date(check_in_time)),
			        COALESCE(SUM(CASE WHEN check_out_time IS NOT NULL
			            THEN (julianday(check_out_time) - julianday(check_in_time)) * 24
			            ELSE 0 END), 0)
			 FROM attendance WHERE student_id = ? AND date(check_in_time) >= ? AND date(check_in_time) <= ?`,
			sid, dr.From, dr.To).Scan(&sp.DaysAttended, &sp.TotalHours)

		if sp.CompletionPct < 50 {
			sp.AtRisk = true
		}

		report.Students = append(report.Students, sp)
		totalCompRate += sp.CompletionPct

		// Operating days for attendance rate
		var opDays int
		db.QueryRow("SELECT COUNT(DISTINCT date(check_in_time)) FROM attendance WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?",
			dr.From, dr.To).Scan(&opDays)
		if opDays > 0 {
			totalAttRate += float64(sp.DaysAttended) / float64(opDays) * 100
		}
	}

	if len(studentIDs) > 0 {
		report.ClassAvgAttendance = totalAttRate / float64(len(studentIDs))
		report.ClassAvgCompletion = totalCompRate / float64(len(studentIDs))
	}

	sortStudentProgress(report.Students)
	return report, nil
}

// --- helpers ---

func sortStudentProgress(students []TeacherStudentProgress) {
	for i := 1; i < len(students); i++ {
		for j := i; j > 0 && students[j].CompletionPct < students[j-1].CompletionPct; j-- {
			students[j], students[j-1] = students[j-1], students[j]
		}
	}
}

func mustParseDate(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, time.Local)
	return t
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
