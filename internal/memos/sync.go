package memos

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"classgo/internal/datastore"
	"classgo/internal/models"
	"classgo/internal/scheduling"
)

// Syncer pushes entity data and attendance summaries into Memos.
type Syncer struct {
	client *Client
	db     *sql.DB
}

// NewSyncer creates a Memos syncer.
func NewSyncer(client *Client, db *sql.DB) *Syncer {
	return &Syncer{client: client, db: db}
}

// SyncAll pushes student profiles, schedule summaries, and today's attendance.
func (s *Syncer) SyncAll() error {
	data, err := datastore.ReadFromDB(s.db)
	if err != nil {
		return fmt.Errorf("read data: %w", err)
	}

	if err := s.syncStudentProfiles(data); err != nil {
		log.Printf("Memos sync students: %v", err)
	}
	if err := s.syncScheduleSummary(data); err != nil {
		log.Printf("Memos sync schedule: %v", err)
	}
	return nil
}

// SyncAttendanceSummary posts a daily attendance summary memo.
func (s *Syncer) SyncAttendanceSummary() error {
	today := time.Now().Format("2006-01-02")
	dayName := time.Now().Format("Monday, January 2")

	rows, err := s.db.Query(
		"SELECT student_name, sign_in_time, sign_out_time FROM attendance WHERE date(sign_in_time) = date('now','localtime') ORDER BY sign_in_time",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var name, signIn string
		var signOut sql.NullString
		if err := rows.Scan(&name, &signIn, &signOut); err != nil {
			continue
		}
		inTime, _ := models.ParseTimestamp(signIn)
		inStr := inTime.Format("3:04 PM")
		if signOut.Valid {
			outTime, _ := models.ParseTimestamp(signOut.String)
			dur := models.FormatDuration(outTime.Sub(inTime))
			lines = append(lines, fmt.Sprintf("- %s: %s - %s (%s)", name, inStr, outTime.Format("3:04 PM"), dur))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: %s - (active)", name, inStr))
		}
	}

	if len(lines) == 0 {
		return nil
	}

	content := fmt.Sprintf("#attendance #%s\n**Attendance: %s**\n\n%s",
		today, dayName, strings.Join(lines, "\n"))

	_, err = s.client.CreateMemo(Memo{
		Content:    content,
		Visibility: "PROTECTED",
	})
	return err
}

func (s *Syncer) syncStudentProfiles(data *datastore.EntityData) error {
	// Build parent lookup
	parentByID := make(map[string]models.Parent)
	for _, p := range data.Parents {
		parentByID[p.ID] = p
	}

	// Build schedule lookup per student
	studentSchedules := make(map[string][]string)
	for _, sched := range data.Schedules {
		for _, sid := range sched.StudentIDs {
			desc := fmt.Sprintf("%s %s-%s %s", sched.DayOfWeek, sched.StartTime, sched.EndTime, sched.Subject)
			if sched.TeacherID != "" {
				desc += " (" + sched.TeacherID + ")"
			}
			studentSchedules[sid] = append(studentSchedules[sid], desc)
		}
	}

	for _, student := range data.Students {
		if !student.Active {
			continue
		}

		var parts []string
		parts = append(parts, fmt.Sprintf("#student #%s", student.ID))
		parts = append(parts, fmt.Sprintf("**%s %s** | Grade %s | %s", student.FirstName, student.LastName, student.Grade, student.School))

		if parent, ok := parentByID[student.ParentID]; ok {
			contact := fmt.Sprintf("Parent: %s %s", parent.FirstName, parent.LastName)
			if parent.Email != "" {
				contact += " (" + parent.Email
				if parent.Phone != "" {
					contact += ", " + parent.Phone
				}
				contact += ")"
			}
			parts = append(parts, contact)
		}

		if scheds, ok := studentSchedules[student.ID]; ok {
			parts = append(parts, "Schedule: "+strings.Join(scheds, " | "))
		}

		if student.Notes != "" {
			parts = append(parts, "Notes: "+student.Notes)
		}

		content := strings.Join(parts, "\n")
		if _, err := s.client.CreateMemo(Memo{
			Content:    content,
			Visibility: "PROTECTED",
			Pinned:     true,
		}); err != nil {
			log.Printf("Memos sync student %s: %v", student.ID, err)
		}
	}
	return nil
}

func (s *Syncer) syncScheduleSummary(data *datastore.EntityData) error {
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfMonth := today.AddDate(0, 1, 0)
	sessions := scheduling.MaterializeSessions(data.Schedules, today, endOfMonth)

	// Group by day of week
	byDay := make(map[string][]scheduling.Session)
	for _, s := range sessions {
		byDay[s.DayOfWeek] = append(byDay[s.DayOfWeek], s)
	}

	// Deduplicate — only show unique schedule patterns per day
	for _, day := range days {
		daySessions := byDay[day]
		if len(daySessions) == 0 {
			continue
		}

		// Deduplicate by schedule ID
		seen := make(map[string]bool)
		var unique []scheduling.Session
		for _, s := range daySessions {
			if !seen[s.ScheduleID] {
				seen[s.ScheduleID] = true
				unique = append(unique, s)
			}
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("#schedule #%s", strings.ToLower(day)))
		lines = append(lines, fmt.Sprintf("**%s Schedule**", day))
		lines = append(lines, "")
		for _, s := range unique {
			line := fmt.Sprintf("- %s-%s | %s | %s",
				s.StartTime, s.EndTime,
				s.RoomID, s.Subject)
			if s.TeacherID != "" {
				line += " | " + s.TeacherID
			}
			if len(s.StudentIDs) > 0 {
				line += " | " + strings.Join(s.StudentIDs, ", ")
			}
			lines = append(lines, line)
		}

		content := strings.Join(lines, "\n")
		if _, err := s.client.CreateMemo(Memo{
			Content:    content,
			Visibility: "PROTECTED",
		}); err != nil {
			log.Printf("Memos sync schedule %s: %v", day, err)
		}
	}
	return nil
}
