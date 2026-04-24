package reports

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"classgo/internal/database"
	"classgo/internal/models"
)

// splitIDs splits a semicolon-or-comma-separated ID string into a slice,
// trimming whitespace and filtering empties.
func splitIDs(s string) []string {
	s = strings.ReplaceAll(s, ",", ";")
	parts := strings.Split(s, ";")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseScheduleHours returns the duration in hours between two time strings (HH:MM).
func parseScheduleHours(startTime, endTime string) float64 {
	start, err1 := time.Parse("15:04", startTime)
	end, err2 := time.Parse("15:04", endTime)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := end.Sub(start)
	if d < 0 {
		d += 24 * time.Hour
	}
	return d.Hours()
}

// DailyAttendanceRecord generates the A1 report for a given date (YYYY-MM-DD).
func DailyAttendanceRecord(db *sql.DB, date string) (*AdminDailyReport, error) {
	metrics, err := database.GetAttendanceMetrics(db, date, date)
	if err != nil {
		return nil, err
	}

	attendees, err := database.AttendeesByDateRange(db, date, date, "", "", "")
	if err != nil {
		return nil, err
	}

	report := &AdminDailyReport{
		Date:            date,
		TotalCheckIns:   metrics.TotalCheckIns,
		UniqueStudents:  metrics.UniqueStudents,
		TotalCheckOuts:  metrics.TotalCheckOuts,
		AvgDurationMins: metrics.AvgDurationMins,
		DeviceBreakdown: make(map[string]int),
	}

	for _, a := range attendees {
		row := AttendanceRow{
			StudentID:   a.StudentID,
			StudentName: a.StudentName,
			DeviceType:  a.DeviceType,
			CheckIn:     a.CheckInTimeStr,
			CheckOut:    a.CheckOutTimeStr,
			Duration:    a.Duration,
			DurationMin: a.DurationMinutes,
		}
		report.Records = append(report.Records, row)
		report.DeviceBreakdown[a.DeviceType]++

		if a.CheckOutTime == nil {
			report.OpenSessions++
			report.OpenSessionList = append(report.OpenSessionList, row)
		}
	}

	// Fraud flags for the day
	flags, _ := database.GetFlaggedAudits(db, date, date)
	report.FraudFlags = len(flags)

	// Compare with same day last week
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err == nil {
		prevDate := t.AddDate(0, 0, -7).Format("2006-01-02")
		prevMetrics, err := database.GetAttendanceMetrics(db, prevDate, prevDate)
		if err == nil {
			report.PrevWeekCompare = &PrevWeekCompare{
				PrevCheckIns:  prevMetrics.TotalCheckIns,
				PrevUnique:    prevMetrics.UniqueStudents,
				DeltaCheckIns: metrics.TotalCheckIns - prevMetrics.TotalCheckIns,
				DeltaUnique:   metrics.UniqueStudents - prevMetrics.UniqueStudents,
			}
		}
	}

	return report, nil
}

// WeeklyCenterPerformance generates the A2 report.
func WeeklyCenterPerformance(db *sql.DB, dr DateRange) (*AdminWeeklyReport, error) {
	metrics, err := database.GetAttendanceMetrics(db, dr.From, dr.To)
	if err != nil {
		return nil, err
	}

	report := &AdminWeeklyReport{
		DateRange:      dr,
		TotalCheckIns:  metrics.TotalCheckIns,
		UniqueStudents: metrics.UniqueStudents,
	}

	if metrics.DayCount > 0 {
		report.AvgDailyAttend = float64(metrics.TotalCheckIns) / float64(metrics.DayCount)
	}

	// Day breakdown
	for _, d := range metrics.ByDay {
		t, _ := time.ParseInLocation("2006-01-02", d.Date, time.Local)
		report.ByDay = append(report.ByDay, DayBreakdown{
			Date:     d.Date,
			DayName:  t.Weekday().String(),
			CheckIns: d.CheckIns,
			AvgMins:  d.AvgMins,
		})
	}

	// Attendance frequency per student
	freqRows, err := db.Query(
		`SELECT student_id, student_name, COUNT(DISTINCT date(check_in_time)) as days
		 FROM attendance WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?
		 GROUP BY student_id`, dr.From, dr.To)
	if err == nil {
		defer freqRows.Close()
		attendedSet := map[string]bool{}
		for freqRows.Next() {
			var sid, sname string
			var days int
			freqRows.Scan(&sid, &sname, &days)
			attendedSet[sid] = true
			if days >= 4 {
				report.ConsistentCount++
			} else if days == 1 {
				report.SporadicCount++
			}
		}

		// No-shows: active students who didn't attend at all
		allIDs, _ := database.GetAllActiveStudentIDs(db)
		for _, id := range allIDs {
			if !attendedSet[id] {
				var fn, ln string
				db.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", id).Scan(&fn, &ln)
				report.NoShows = append(report.NoShows, StudentBasic{ID: id, Name: fn + " " + ln})
			}
		}
	}

	// New students (first ever attendance this week)
	newRows, err := db.Query(
		`SELECT student_id, student_name FROM attendance
		 WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?
		 GROUP BY student_id
		 HAVING MIN(date(check_in_time)) = (SELECT MIN(date(check_in_time)) FROM attendance a2 WHERE a2.student_id = attendance.student_id)`,
		dr.From, dr.To)
	if err == nil {
		defer newRows.Close()
		for newRows.Next() {
			var s StudentBasic
			newRows.Scan(&s.ID, &s.Name)
			report.NewStudents = append(report.NewStudents, s)
		}
	}

	// Week-over-week comparison
	fromT, _ := time.ParseInLocation("2006-01-02", dr.From, time.Local)
	prevFrom := fromT.AddDate(0, 0, -7).Format("2006-01-02")
	prevTo := fromT.AddDate(0, 0, -1).Format("2006-01-02")
	prevMetrics, err := database.GetAttendanceMetrics(db, prevFrom, prevTo)
	if err == nil {
		report.PrevWeek = &PrevWeekCompare{
			PrevCheckIns:  prevMetrics.TotalCheckIns,
			PrevUnique:    prevMetrics.UniqueStudents,
			DeltaCheckIns: metrics.TotalCheckIns - prevMetrics.TotalCheckIns,
			DeltaUnique:   metrics.UniqueStudents - prevMetrics.UniqueStudents,
		}
	}

	// Center-wide completion rate
	allIDs, _ := database.GetAllActiveStudentIDs(db)
	if len(allIDs) > 0 {
		stats, _ := database.GetProgressStats(db, allIDs, dr.From, dr.To)
		var totalDone, totalItems int
		for _, s := range stats {
			totalDone += s.DoneCount
			totalItems += s.TotalItems
		}
		if totalItems > 0 {
			report.CompletionRate = float64(totalDone) / float64(totalItems) * 100
		}
	}

	// Fraud flags
	flags, _ := database.GetFlaggedAudits(db, dr.From, dr.To)
	report.FraudFlagCount = len(flags)

	return report, nil
}

// TeacherWorkloadReport generates the A3 report.
func TeacherWorkloadReport(db *sql.DB, dr DateRange) (*AdminTeacherWorkload, error) {
	report := &AdminTeacherWorkload{DateRange: dr}

	// Get all active teachers
	teacherRows, err := db.Query(
		"SELECT id, first_name, last_name FROM teachers WHERE active = 1 AND deleted = 0 ORDER BY first_name, last_name")
	if err != nil {
		return nil, err
	}
	defer teacherRows.Close()

	for teacherRows.Next() {
		var tid, fn, ln string
		teacherRows.Scan(&tid, &fn, &ln)

		row := TeacherWorkloadRow{
			TeacherID:   tid,
			TeacherName: fn + " " + ln,
		}

		// Count classes and enrolled students
		schedRows, err := db.Query(
			"SELECT COALESCE(student_ids,''), start_time, end_time FROM schedules WHERE teacher_id = ? AND deleted = 0", tid)
		if err == nil {
			enrolledSet := map[string]bool{}
			for schedRows.Next() {
				var sids, startT, endT string
				schedRows.Scan(&sids, &startT, &endT)
				row.ClassCount++
				// Parse schedule duration
				row.TotalHours += parseScheduleHours(startT, endT)
				for _, sid := range splitIDs(sids) {
					enrolledSet[sid] = true
				}
			}
			schedRows.Close()
			row.EnrolledCount = len(enrolledSet)
		}

		// Actual attendance for this teacher's students
		var attended int
		db.QueryRow(
			`SELECT COUNT(DISTINCT am.student_id)
			 FROM attendance_meta am
			 JOIN attendance a ON a.id = am.attendance_id
			 JOIN schedules s ON s.id = am.schedule_id
			 WHERE s.teacher_id = ? AND date(a.check_in_time) >= ? AND date(a.check_in_time) <= ?`,
			tid, dr.From, dr.To).Scan(&attended)
		row.AttendedCount = attended

		// Tasks created
		var tasksCreated int
		db.QueryRow(
			`SELECT COUNT(*) FROM task_items WHERE created_by = ? AND date(created_at) >= ? AND date(created_at) <= ?`,
			tid, dr.From, dr.To).Scan(&tasksCreated)
		row.TasksCreated = tasksCreated

		report.Teachers = append(report.Teachers, row)
	}

	// Room utilization
	roomRows, err := db.Query("SELECT id, name, COALESCE(capacity, 0) FROM rooms WHERE deleted = 0 ORDER BY name")
	if err == nil {
		defer roomRows.Close()
		for roomRows.Next() {
			var rid, rname string
			var cap int
			roomRows.Scan(&rid, &rname, &cap)

			row := RoomUtilizationRow{
				RoomID:   rid,
				RoomName: rname,
				Capacity: cap,
			}

			// Average enrolled per schedule in this room
			var totalEnrolled, schedCount int
			sRows, _ := db.Query("SELECT COALESCE(student_ids,'') FROM schedules WHERE room_id = ? AND deleted = 0", rid)
			if sRows != nil {
				for sRows.Next() {
					var sids string
					sRows.Scan(&sids)
					schedCount++
					totalEnrolled += len(splitIDs(sids))
				}
				sRows.Close()
			}
			if schedCount > 0 {
				row.AvgEnrolled = float64(totalEnrolled) / float64(schedCount)
			}
			if cap > 0 {
				row.Utilization = row.AvgEnrolled / float64(cap) * 100
			}
			report.Rooms = append(report.Rooms, row)
		}
	}

	return report, nil
}

// MonthlyDashboard generates the A4 report.
func MonthlyDashboard(db *sql.DB, dr DateRange) (*AdminMonthlyReport, error) {
	metrics, err := database.GetAttendanceMetrics(db, dr.From, dr.To)
	if err != nil {
		return nil, err
	}

	report := &AdminMonthlyReport{
		DateRange:      dr,
		TotalCheckIns:  metrics.TotalCheckIns,
		UniqueStudents: metrics.UniqueStudents,
	}
	if metrics.DayCount > 0 {
		report.AvgDailyAttend = float64(metrics.TotalCheckIns) / float64(metrics.DayCount)
	}

	// Weekly breakdown within the month
	fromT, _ := time.ParseInLocation("2006-01-02", dr.From, time.Local)
	toT, _ := time.ParseInLocation("2006-01-02", dr.To, time.Local)
	weekNum := 1
	for wStart := fromT; wStart.Before(toT) || wStart.Equal(toT); {
		wEnd := wStart.AddDate(0, 0, 6)
		if wEnd.After(toT) {
			wEnd = toT
		}
		wFrom := wStart.Format("2006-01-02")
		wTo := wEnd.Format("2006-01-02")

		wm, _ := database.GetAttendanceMetrics(db, wFrom, wTo)
		wb := WeekBreakdown{
			WeekLabel: fmt.Sprintf("Week %d", weekNum),
			From:      wFrom,
			To:        wTo,
		}
		if wm != nil {
			wb.CheckIns = wm.TotalCheckIns
			wb.Unique = wm.UniqueStudents
		}
		report.WeeklyBreakdown = append(report.WeeklyBreakdown, wb)
		wStart = wEnd.AddDate(0, 0, 1)
		weekNum++
	}

	// New students (first ever attendance in this month)
	db.QueryRow(
		`SELECT COUNT(DISTINCT student_id) FROM attendance
		 WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?
		 AND student_id NOT IN (
			SELECT DISTINCT student_id FROM attendance WHERE date(check_in_time) < ?
		 )`, dr.From, dr.To, dr.From).Scan(&report.NewStudents)

	// Inactive: active students with no attendance this month
	var totalActive int
	db.QueryRow("SELECT COUNT(*) FROM students WHERE active = 1 AND deleted = 0").Scan(&totalActive)
	report.InactiveStudents = totalActive - metrics.UniqueStudents

	// Completion rate
	allIDs, _ := database.GetAllActiveStudentIDs(db)
	if len(allIDs) > 0 {
		stats, _ := database.GetProgressStats(db, allIDs, dr.From, dr.To)
		var totalDone, totalItems int
		for _, s := range stats {
			totalDone += s.DoneCount
			totalItems += s.TotalItems
		}
		if totalItems > 0 {
			report.CompletionRate = float64(totalDone) / float64(totalItems) * 100
		}
	}

	// Profile gaps
	report.ProfileGaps = getProfileGaps(db)

	// Fraud
	flags, _ := database.GetFlaggedAudits(db, dr.From, dr.To)
	report.FraudFlagCount = len(flags)

	return report, nil
}

// EngagementScorecard generates the A5 report.
func EngagementScorecard(db *sql.DB, dr DateRange) (*AdminEngagementReport, error) {
	report := &AdminEngagementReport{DateRange: dr}

	allIDs, err := database.GetAllActiveStudentIDs(db)
	if err != nil {
		return nil, err
	}

	// Operating days in range
	var operatingDays int
	db.QueryRow(
		"SELECT COUNT(DISTINCT date(check_in_time)) FROM attendance WHERE date(check_in_time) >= ? AND date(check_in_time) <= ?",
		dr.From, dr.To).Scan(&operatingDays)
	if operatingDays == 0 {
		operatingDays = 1
	}

	// Target session duration (e.g. 120 minutes)
	targetDuration := 120.0

	// Get progress stats
	progressMap := map[string]models.ProgressStats{}
	stats, _ := database.GetProgressStats(db, allIDs, dr.From, dr.To)
	for _, s := range stats {
		progressMap[s.StudentID] = s
	}

	var totalScore float64
	for _, sid := range allIDs {
		var fn, ln, grade, school string
		db.QueryRow("SELECT first_name, last_name, COALESCE(grade,''), COALESCE(school,'') FROM students WHERE id = ?", sid).
			Scan(&fn, &ln, &grade, &school)

		// Attendance score: days attended / operating days
		var daysAttended int
		db.QueryRow(
			"SELECT COUNT(DISTINCT date(check_in_time)) FROM attendance WHERE student_id = ? AND date(check_in_time) >= ? AND date(check_in_time) <= ?",
			sid, dr.From, dr.To).Scan(&daysAttended)
		attScore := clamp(float64(daysAttended)/float64(operatingDays)*100, 0, 100)

		// Completion score
		compScore := 0.0
		if p, ok := progressMap[sid]; ok && p.TotalItems > 0 {
			compScore = p.Completion
		}

		// Duration score: avg session / target
		var avgDuration float64
		db.QueryRow(
			`SELECT COALESCE(AVG(CASE WHEN check_out_time IS NOT NULL
				THEN (julianday(check_out_time) - julianday(check_in_time)) * 24 * 60
				ELSE NULL END), 0)
			 FROM attendance WHERE student_id = ? AND date(check_in_time) >= ? AND date(check_in_time) <= ?`,
			sid, dr.From, dr.To).Scan(&avgDuration)
		durScore := clamp(avgDuration/targetDuration*100, 0, 100)

		composite := attScore*0.4 + compScore*0.4 + durScore*0.2

		score := EngagementScore{
			StudentID:       sid,
			StudentName:     fn + " " + ln,
			Grade:           grade,
			School:          school,
			AttendanceScore: attScore,
			CompletionScore: compScore,
			DurationScore:   durScore,
			CompositeScore:  composite,
		}

		report.Students = append(report.Students, score)
		totalScore += composite
	}

	// Mark bottom quartile as at-risk
	if len(report.Students) > 0 {
		report.AvgScore = totalScore / float64(len(report.Students))
		// Sort by composite score ascending to find bottom quartile
		sortStudentsByScore(report.Students)
		cutoff := len(report.Students) / 4
		if cutoff < 1 {
			cutoff = 1
		}
		for i := 0; i < cutoff && i < len(report.Students); i++ {
			report.Students[i].AtRisk = true
			report.AtRiskCount++
		}
	}

	return report, nil
}

// AuditComplianceLog generates the A6 report.
func AuditComplianceLog(db *sql.DB, dr DateRange) (*AdminAuditReport, error) {
	report := &AdminAuditReport{DateRange: dr}

	flags, err := database.GetFlaggedAudits(db, dr.From, dr.To)
	if err != nil {
		return nil, err
	}

	report.TotalFlags = len(flags)
	for _, f := range flags {
		report.FlaggedEvents = append(report.FlaggedEvents, AuditEvent{
			StudentName: f.StudentName,
			StudentID:   f.StudentID,
			DeviceType:  f.DeviceType,
			ClientIP:    f.ClientIP,
			FlagReason:  f.FlagReason,
			CreatedAt:   f.CreatedAt,
		})
	}

	// Repeat offenders
	offRows, err := db.Query(
		`SELECT student_name, COALESCE(student_id,''), COUNT(*) as cnt
		 FROM checkin_audit
		 WHERE flagged = 1 AND date(created_at) >= ? AND date(created_at) <= ?
		 GROUP BY student_id HAVING cnt > 1
		 ORDER BY cnt DESC`, dr.From, dr.To)
	if err == nil {
		defer offRows.Close()
		for offRows.Next() {
			var o RepeatOffender
			offRows.Scan(&o.StudentName, &o.StudentID, &o.FlagCount)
			report.RepeatOffenders = append(report.RepeatOffenders, o)
		}
	}

	return report, nil
}

// --- helpers ---

func getProfileGaps(db *sql.DB) ProfileGapSummary {
	var g ProfileGapSummary
	db.QueryRow("SELECT COUNT(*) FROM students WHERE active = 1 AND deleted = 0").Scan(&g.TotalStudents)
	db.QueryRow("SELECT COUNT(*) FROM students WHERE active = 1 AND deleted = 0 AND (grade IS NULL OR grade = '')").Scan(&g.MissingGrade)
	db.QueryRow("SELECT COUNT(*) FROM students WHERE active = 1 AND deleted = 0 AND (parent_id IS NULL OR parent_id = '')").Scan(&g.MissingParent)
	db.QueryRow("SELECT COUNT(*) FROM students WHERE active = 1 AND deleted = 0 AND (dob IS NULL OR dob = '')").Scan(&g.MissingDOB)
	db.QueryRow("SELECT COUNT(*) FROM parents WHERE deleted = 0 AND (email IS NULL OR email = '')").Scan(&g.ParentsNoEmail)
	return g
}

func sortStudentsByScore(students []EngagementScore) {
	for i := 1; i < len(students); i++ {
		for j := i; j > 0 && students[j].CompositeScore < students[j-1].CompositeScore; j-- {
			students[j], students[j-1] = students[j-1], students[j]
		}
	}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
