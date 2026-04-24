package scheduling

import (
	"strings"
	"time"

	"classgo/internal/models"
)

// Session represents a materialized instance of a schedule template on a specific date.
type Session struct {
	ScheduleID string    `json:"schedule_id"`
	Date       time.Time `json:"date"`
	DateStr    string    `json:"date_str"`
	DayOfWeek  string    `json:"day_of_week"`
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	TeacherID  string    `json:"teacher_id"`
	RoomID     string    `json:"room_id"`
	Subject    string    `json:"subject"`
	StudentIDs []string  `json:"student_ids"`
	Type       string    `json:"type"`
}

// MaterializeSessions generates concrete session instances from schedule templates
// for the given date range. Each schedule template with a matching day_of_week
// produces a session for each matching calendar date in the range.
func MaterializeSessions(schedules []models.Schedule, from, to time.Time) []Session {
	var sessions []Session

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dayName := d.Weekday().String() // "Monday", "Tuesday", etc.

		for _, sched := range schedules {
			if !strings.EqualFold(sched.DayOfWeek, dayName) {
				continue
			}

			// Check effective date range
			if sched.EffectiveFrom != "" {
				ef, err := time.Parse("2006-01-02", sched.EffectiveFrom)
				if err == nil && d.Before(ef) {
					continue
				}
			}
			if sched.EffectiveUntil != "" {
				eu, err := time.Parse("2006-01-02", sched.EffectiveUntil)
				if err == nil && d.After(eu) {
					continue
				}
			}

			sessions = append(sessions, Session{
				ScheduleID: sched.ID,
				Date:       d,
				DateStr:    d.Format("2006-01-02"),
				DayOfWeek:  dayName,
				StartTime:  sched.StartTime,
				EndTime:    sched.EndTime,
				TeacherID:  sched.TeacherID,
				RoomID:     sched.RoomID,
				Subject:    sched.Subject,
				StudentIDs: sched.StudentIDs,
				Type:       sched.Type,
			})
		}
	}

	return sessions
}

// TodaySessions returns materialized sessions for today.
func TodaySessions(schedules []models.Schedule) []Session {
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return MaterializeSessions(schedules, today, today)
}

// WeekSessions returns materialized sessions for the current week (Monday-Sunday).
func WeekSessions(schedules []models.Schedule) []Session {
	now := time.Now()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-time.Monday))
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
	sunday := monday.AddDate(0, 0, 6)
	return MaterializeSessions(schedules, monday, sunday)
}
