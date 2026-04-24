package reports

import (
	"database/sql"
	"strings"
	"time"

	"classgo/internal/database"
	"classgo/internal/models"
	"classgo/internal/scheduling"
)

// TypeHours represents hours for a specific schedule type.
type TypeHours struct {
	Type  string  `json:"type"`
	Hours float64 `json:"hours"`
}

// TimesheetDay represents a single day in the timesheet.
type TimesheetDay struct {
	Date           string           `json:"date"`
	DayOfWeek      string           `json:"day_of_week"`
	HoursByType    []TypeHours      `json:"hours_by_type"`
	ScheduledHours float64          `json:"scheduled_hours"`
	TimeOff        []models.TimeOff `json:"time_off,omitempty"`
	NetHours       float64          `json:"net_hours"`
}

// WeekSummary groups timesheet data by ISO week.
type WeekSummary struct {
	WeekNumber     int         `json:"week_number"`
	StartDate      string      `json:"start_date"`
	EndDate        string      `json:"end_date"`
	HoursByType    []TypeHours `json:"hours_by_type"`
	ScheduledHours float64     `json:"scheduled_hours"`
	TimeOffHours   float64     `json:"time_off_hours"`
	NetHours       float64     `json:"net_hours"`
}

// StaffTimesheetReport is the timesheet for a single teacher/staff member.
type StaffTimesheetReport struct {
	TeacherID      string         `json:"teacher_id"`
	TeacherName    string         `json:"teacher_name"`
	Period         string         `json:"period"`
	Days           []TimesheetDay `json:"days"`
	TotalByType    []TypeHours    `json:"total_by_type"`
	TotalScheduled float64        `json:"total_scheduled"`
	TotalTimeOff   float64        `json:"total_time_off"`
	NetHours       float64        `json:"net_hours"`
	WeeklySummary  []WeekSummary  `json:"weekly_summary"`
}

// AdminTimesheetReport aggregates timesheets for all staff.
type AdminTimesheetReport struct {
	Period string                 `json:"period"`
	Staff  []StaffTimesheetReport `json:"staff"`
}

// StaffTimesheet generates a timesheet report for a single teacher.
func StaffTimesheet(db *sql.DB, dr DateRange, teacherID string) (*StaffTimesheetReport, error) {
	// Look up teacher name
	var firstName, lastName string
	_ = db.QueryRow("SELECT COALESCE(first_name,''), COALESCE(last_name,'') FROM teachers WHERE id = ?", teacherID).Scan(&firstName, &lastName)
	teacherName := firstName
	if lastName != "" {
		teacherName += " " + lastName
	}

	// Query schedules for this teacher
	schedules, err := getTeacherSchedules(db, teacherID)
	if err != nil {
		return nil, err
	}

	from, _ := time.Parse("2006-01-02", dr.From)
	to, _ := time.Parse("2006-01-02", dr.To)

	// Materialize sessions
	sessions := scheduling.MaterializeSessions(schedules, from, to)

	// Build date -> type -> hours map
	dateTypeHours := make(map[string]map[string]float64)
	for _, s := range sessions {
		typ := s.Type
		if typ == "" {
			typ = "class"
		}
		if dateTypeHours[s.DateStr] == nil {
			dateTypeHours[s.DateStr] = make(map[string]float64)
		}
		dateTypeHours[s.DateStr][typ] += parseScheduleHours(s.StartTime, s.EndTime)
	}

	// Query time off
	timeoffs, _ := database.ListTimeOff(db, teacherID, "teacher", dr.From, dr.To)
	// Build date -> []TimeOff map
	dateTimeOff := make(map[string][]models.TimeOff)
	for _, to := range timeoffs {
		dateTimeOff[to.Date] = append(dateTimeOff[to.Date], to)
	}

	// Build daily breakdown
	var days []TimesheetDay
	typeTotals := make(map[string]float64)
	var totalScheduled, totalTimeOff float64

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		dayHours := dateTypeHours[dateStr] // may be nil
		dayTOs := dateTimeOff[dateStr]

		// Compute scheduled hours by type for this day
		dayTyped := make(map[string]float64)
		var dayTotal float64
		for typ, h := range dayHours {
			dayTyped[typ] = h
			dayTotal += h
		}

		// Apply time-off deductions
		var dayTimeOffHours float64
		for _, to := range dayTOs {
			if to.ScheduleType == "" {
				// All types
				if to.Hours == 0 {
					// Full day: zero out all types
					for typ := range dayTyped {
						dayTimeOffHours += dayTyped[typ]
						dayTyped[typ] = 0
					}
				} else {
					// Proportional deduction across types
					dayTimeOffHours += to.Hours
					remaining := to.Hours
					for typ := range dayTyped {
						if remaining <= 0 {
							break
						}
						deduct := remaining
						if deduct > dayTyped[typ] {
							deduct = dayTyped[typ]
						}
						dayTyped[typ] -= deduct
						remaining -= deduct
					}
				}
			} else {
				// Specific schedule type
				if to.Hours == 0 {
					dayTimeOffHours += dayTyped[to.ScheduleType]
					dayTyped[to.ScheduleType] = 0
				} else {
					deduct := to.Hours
					if deduct > dayTyped[to.ScheduleType] {
						deduct = dayTyped[to.ScheduleType]
					}
					dayTyped[to.ScheduleType] -= deduct
					dayTimeOffHours += deduct
				}
			}
		}

		// Build TypeHours slice
		var hoursByType []TypeHours
		var netHours float64
		for typ, h := range dayTyped {
			if h > 0 || dayHours[typ] > 0 {
				hoursByType = append(hoursByType, TypeHours{Type: typ, Hours: h})
			}
			netHours += h
			typeTotals[typ] += h
		}

		totalScheduled += dayTotal
		totalTimeOff += dayTimeOffHours

		days = append(days, TimesheetDay{
			Date:           dateStr,
			DayOfWeek:      d.Weekday().String(),
			HoursByType:    hoursByType,
			ScheduledHours: dayTotal,
			TimeOff:        dayTOs,
			NetHours:       netHours,
		})
	}

	// Build weekly summaries
	weekMap := make(map[int]*WeekSummary)
	var weekOrder []int
	for _, day := range days {
		d, _ := time.Parse("2006-01-02", day.Date)
		_, wk := d.ISOWeek()
		ws, ok := weekMap[wk]
		if !ok {
			monday := d.AddDate(0, 0, -int(d.Weekday()-time.Monday))
			if d.Weekday() == time.Sunday {
				monday = d.AddDate(0, 0, -6)
			}
			ws = &WeekSummary{
				WeekNumber: wk,
				StartDate:  monday.Format("2006-01-02"),
				EndDate:    monday.AddDate(0, 0, 6).Format("2006-01-02"),
			}
			weekMap[wk] = ws
			weekOrder = append(weekOrder, wk)
		}
		ws.ScheduledHours += day.ScheduledHours
		ws.NetHours += day.NetHours
		ws.TimeOffHours += day.ScheduledHours - day.NetHours
		for _, th := range day.HoursByType {
			found := false
			for i := range ws.HoursByType {
				if ws.HoursByType[i].Type == th.Type {
					ws.HoursByType[i].Hours += th.Hours
					found = true
					break
				}
			}
			if !found {
				ws.HoursByType = append(ws.HoursByType, TypeHours{Type: th.Type, Hours: th.Hours})
			}
		}
	}
	var weeklySummary []WeekSummary
	for _, wk := range weekOrder {
		weeklySummary = append(weeklySummary, *weekMap[wk])
	}

	// Build total by type
	var totalByType []TypeHours
	for typ, h := range typeTotals {
		totalByType = append(totalByType, TypeHours{Type: typ, Hours: h})
	}

	return &StaffTimesheetReport{
		TeacherID:      teacherID,
		TeacherName:    teacherName,
		Period:         dr.From + " to " + dr.To,
		Days:           days,
		TotalByType:    totalByType,
		TotalScheduled: totalScheduled,
		TotalTimeOff:   totalTimeOff,
		NetHours:       totalScheduled - totalTimeOff,
		WeeklySummary:  weeklySummary,
	}, nil
}

// AdminStaffTimesheet generates timesheets for all active teachers.
func AdminStaffTimesheet(db *sql.DB, dr DateRange) (*AdminTimesheetReport, error) {
	rows, err := db.Query("SELECT id FROM teachers WHERE active = 1 AND deleted = 0 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var staff []StaffTimesheetReport
	for rows.Next() {
		var teacherID string
		if err := rows.Scan(&teacherID); err != nil {
			continue
		}
		report, err := StaffTimesheet(db, dr, teacherID)
		if err != nil {
			continue
		}
		staff = append(staff, *report)
	}

	return &AdminTimesheetReport{
		Period: dr.From + " to " + dr.To,
		Staff:  staff,
	}, nil
}

// getTeacherSchedules queries non-deleted schedules for a specific teacher.
func getTeacherSchedules(db *sql.DB, teacherID string) ([]models.Schedule, error) {
	rows, err := db.Query(
		`SELECT id, day_of_week, start_time, end_time, COALESCE(teacher_id,''), COALESCE(room_id,''),
		        COALESCE(subject,''), COALESCE(student_ids,''), COALESCE(effective_from,''),
		        COALESCE(effective_until,''), COALESCE(type,'class')
		 FROM schedules WHERE teacher_id = ? AND deleted = 0`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var studentIDs string
		if err := rows.Scan(&s.ID, &s.DayOfWeek, &s.StartTime, &s.EndTime, &s.TeacherID,
			&s.RoomID, &s.Subject, &studentIDs, &s.EffectiveFrom, &s.EffectiveUntil, &s.Type); err != nil {
			return nil, err
		}
		if studentIDs != "" {
			for _, id := range splitSemicolon(studentIDs) {
				s.StudentIDs = append(s.StudentIDs, id)
			}
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func splitSemicolon(s string) []string {
	var result []string
	for _, p := range strings.Split(s, ";") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
