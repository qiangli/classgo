package reports

import (
	"database/sql"

	"classgo/internal/database"
	"classgo/internal/models"
)

// ChildAttendanceProgress generates the P1 report for a parent.
func ChildAttendanceProgress(db *sql.DB, dr DateRange, parentID string) (*ParentChildReport, error) {
	report := &ParentChildReport{
		DateRange: dr,
		ParentID:  parentID,
	}

	db.QueryRow("SELECT first_name || ' ' || last_name FROM parents WHERE id = ?", parentID).Scan(&report.ParentName)

	childIDs, err := database.GetParentStudentIDs(db, parentID)
	if err != nil {
		return nil, err
	}

	// Progress stats for all children
	progressMap := map[string]models.ProgressStats{}
	stats, _ := database.GetProgressStats(db, childIDs, dr.From, dr.To)
	for _, s := range stats {
		progressMap[s.StudentID] = s
	}

	for _, sid := range childIDs {
		var fn, ln string
		db.QueryRow("SELECT first_name, last_name FROM students WHERE id = ?", sid).Scan(&fn, &ln)

		child := ChildSummary{
			StudentID:   sid,
			StudentName: fn + " " + ln,
		}

		// Attendance details
		attendees, _ := database.AttendeesByDateRange(db, dr.From, dr.To, sid, "", "")
		daysSet := map[string]bool{}
		for _, a := range attendees {
			daysSet[a.Date] = true
			child.TotalHours += a.DurationMinutes / 60
			child.DailyDetail = append(child.DailyDetail, ChildDayDetail{
				Date:     a.Date,
				CheckIn:  a.CheckInTimeStr,
				CheckOut: a.CheckOutTimeStr,
				Duration: a.Duration,
			})
		}
		child.DaysAttended = len(daysSet)

		// Completion
		if p, ok := progressMap[sid]; ok {
			child.CompletionPct = p.Completion
		}

		// Engagement level
		child.EngagementLevel = engagementLabel(child.CompletionPct)

		// Incomplete items
		respRows, _ := db.Query(
			`SELECT item_name, status, COALESCE(is_late, 0) FROM tracker_responses
			 WHERE student_id = ? AND status = 'not_done'
			 AND response_date >= ? AND response_date <= ?
			 ORDER BY response_date DESC`,
			sid, dr.From, dr.To)
		if respRows != nil {
			for respRows.Next() {
				var it IncompleteItem
				respRows.Scan(&it.Name, &it.Status, &it.IsLate)
				child.IncompleteItems = append(child.IncompleteItems, it)
			}
			respRows.Close()
		}

		// Upcoming tasks (next 7 days from range end)
		toDate := mustParseDate(dr.To)
		futureDate := toDate.AddDate(0, 0, 7).Format("2006-01-02")
		taskRows, _ := db.Query(
			`SELECT name, COALESCE(end_date,''), priority FROM task_items
			 WHERE active = 1 AND deleted = 0 AND completed = 0
			 AND (scope = 1 OR (scope = 3 AND student_id = ?))
			 AND end_date > ? AND end_date <= ?
			 ORDER BY end_date, priority`,
			sid, dr.To, futureDate)
		if taskRows != nil {
			for taskRows.Next() {
				var t UpcomingTask
				taskRows.Scan(&t.Name, &t.DueDate, &t.Priority)
				child.UpcomingTasks = append(child.UpcomingTasks, t)
			}
			taskRows.Close()
		}

		report.Children = append(report.Children, child)
	}

	return report, nil
}

func engagementLabel(completionPct float64) string {
	if completionPct >= 80 {
		return "Excellent"
	}
	if completionPct >= 50 {
		return "Good"
	}
	return "Needs Attention"
}
