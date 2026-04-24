package reports

import (
	"database/sql"

	"classgo/internal/database"
)

// MyWeeklySummary generates the S1 report for a student.
func MyWeeklySummary(db *sql.DB, dr DateRange, studentID string) (*StudentWeeklyReport, error) {
	report := &StudentWeeklyReport{
		DateRange: dr,
		StudentID: studentID,
	}

	db.QueryRow("SELECT first_name || ' ' || last_name FROM students WHERE id = ?", studentID).Scan(&report.StudentName)

	// Attendance
	db.QueryRow(
		`SELECT COUNT(DISTINCT date(check_in_time)),
		        COALESCE(SUM(CASE WHEN check_out_time IS NOT NULL
		            THEN (julianday(check_out_time) - julianday(check_in_time)) * 24
		            ELSE 0 END), 0)
		 FROM attendance WHERE student_id = ? AND date(check_in_time) >= ? AND date(check_in_time) <= ?`,
		studentID, dr.From, dr.To).Scan(&report.DaysAttended, &report.TotalHours)

	// Task completion
	stats, _ := database.GetProgressStats(db, []string{studentID}, dr.From, dr.To)
	if len(stats) > 0 {
		report.TasksCompleted = stats[0].DoneCount
		report.TasksTotal = stats[0].TotalItems
		report.CompletionPct = stats[0].Completion
	}

	// Late count
	db.QueryRow(
		`SELECT COUNT(*) FROM tracker_responses
		 WHERE student_id = ? AND is_late = 1
		 AND response_date >= ? AND response_date <= ?`,
		studentID, dr.From, dr.To).Scan(&report.LateCount)

	// Attendance streak (consecutive days ending at the report end date)
	report.Streak = attendanceStreak(db, studentID, dr.To)

	return report, nil
}

// MyMonthlyProgress generates the S2 report for a student.
func MyMonthlyProgress(db *sql.DB, dr DateRange, studentID string) (*StudentMonthlyReport, error) {
	report := &StudentMonthlyReport{
		DateRange: dr,
		StudentID: studentID,
	}

	db.QueryRow("SELECT first_name || ' ' || last_name FROM students WHERE id = ?", studentID).Scan(&report.StudentName)

	// Attendance
	db.QueryRow(
		`SELECT COUNT(DISTINCT date(check_in_time)),
		        COALESCE(SUM(CASE WHEN check_out_time IS NOT NULL
		            THEN (julianday(check_out_time) - julianday(check_in_time)) * 24
		            ELSE 0 END), 0)
		 FROM attendance WHERE student_id = ? AND date(check_in_time) >= ? AND date(check_in_time) <= ?`,
		studentID, dr.From, dr.To).Scan(&report.DaysAttended, &report.TotalHours)

	// Overall completion
	stats, _ := database.GetProgressStats(db, []string{studentID}, dr.From, dr.To)
	if len(stats) > 0 {
		report.CompletionPct = stats[0].Completion
	}

	report.EngagementLevel = engagementLabel(report.CompletionPct)

	// Category breakdown
	catRows, err := db.Query(
		`SELECT COALESCE(ti.category, 'Uncategorized'),
		        COUNT(*) as total,
		        SUM(CASE WHEN tr.status = 'done' THEN 1 ELSE 0 END) as done
		 FROM tracker_responses tr
		 JOIN task_items ti ON ti.id = tr.item_id
		 WHERE tr.student_id = ? AND tr.response_date >= ? AND tr.response_date <= ?
		 GROUP BY ti.category`,
		studentID, dr.From, dr.To)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat string
			var total, done int
			catRows.Scan(&cat, &total, &done)
			pct := 0.0
			if total > 0 {
				pct = float64(done) / float64(total) * 100
			}
			report.CategoryBreakdown = append(report.CategoryBreakdown, CategoryCompletion{
				Category:      cat,
				CompletionPct: pct,
			})
		}
	}

	// Weekly trend within the month
	fromT := mustParseDate(dr.From)
	toT := mustParseDate(dr.To)
	weekNum := 1
	for wStart := fromT; !wStart.After(toT); {
		wEnd := wStart.AddDate(0, 0, 6)
		if wEnd.After(toT) {
			wEnd = toT
		}
		wFrom := wStart.Format("2006-01-02")
		wTo := wEnd.Format("2006-01-02")

		wStats, _ := database.GetProgressStats(db, []string{studentID}, wFrom, wTo)
		pct := 0.0
		if len(wStats) > 0 {
			pct = wStats[0].Completion
		}
		report.WeeklyTrend = append(report.WeeklyTrend, WeekCompletionRow{
			WeekLabel:     "Week " + itoa(weekNum),
			CompletionPct: pct,
		})

		wStart = wEnd.AddDate(0, 0, 1)
		weekNum++
	}

	return report, nil
}

// attendanceStreak counts consecutive attendance days ending at the given date.
func attendanceStreak(db *sql.DB, studentID, endDate string) int {
	rows, err := db.Query(
		`SELECT DISTINCT date(check_in_time) FROM attendance
		 WHERE student_id = ? AND date(check_in_time) <= ?
		 ORDER BY date(check_in_time) DESC LIMIT 60`,
		studentID, endDate)
	if err != nil {
		return 0
	}
	defer rows.Close()

	streak := 0
	expected := mustParseDate(endDate)
	for rows.Next() {
		var d string
		rows.Scan(&d)
		actual := mustParseDate(d)
		if actual.Equal(expected) {
			streak++
			expected = expected.AddDate(0, 0, -1)
		} else if actual.Before(expected) {
			break
		}
	}
	return streak
}
