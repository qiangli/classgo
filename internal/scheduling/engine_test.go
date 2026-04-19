package scheduling

import (
	"testing"
	"time"

	"classgo/internal/models"
)

func testSchedules() []models.Schedule {
	return []models.Schedule{
		{ID: "SCH001", DayOfWeek: "Monday", StartTime: "15:30", EndTime: "17:00", TeacherID: "T01", RoomID: "R01", Subject: "math", StudentIDs: []string{"S001", "S002"}, EffectiveFrom: "2026-01-06"},
		{ID: "SCH002", DayOfWeek: "Wednesday", StartTime: "15:30", EndTime: "17:00", TeacherID: "T01", RoomID: "R01", Subject: "math", StudentIDs: []string{"S001", "S002"}, EffectiveFrom: "2026-01-06"},
		{ID: "SCH003", DayOfWeek: "Tuesday", StartTime: "16:00", EndTime: "17:30", TeacherID: "T02", RoomID: "R02", Subject: "english", StudentIDs: []string{"S003"}, EffectiveFrom: "2026-01-06"},
	}
}

func TestMaterializeSessions(t *testing.T) {
	schedules := testSchedules()

	// One week starting Monday 2026-01-12
	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 1, 18, 0, 0, 0, 0, time.Local) // Sunday

	sessions := MaterializeSessions(schedules, from, to)

	// Should get: Monday(SCH001), Tuesday(SCH003), Wednesday(SCH002) = 3
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Check Monday session
	if sessions[0].ScheduleID != "SCH001" || sessions[0].DayOfWeek != "Monday" {
		t.Errorf("first session should be SCH001 on Monday, got %s on %s", sessions[0].ScheduleID, sessions[0].DayOfWeek)
	}
	if sessions[0].DateStr != "2026-01-12" {
		t.Errorf("expected date 2026-01-12, got %s", sessions[0].DateStr)
	}
}

func TestMaterializeSessionsEffectiveDates(t *testing.T) {
	schedules := []models.Schedule{
		{ID: "S1", DayOfWeek: "Monday", StartTime: "10:00", EndTime: "11:00", EffectiveFrom: "2026-02-01", EffectiveUntil: "2026-02-28"},
	}

	// Before effective range
	from := time.Date(2026, 1, 19, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 1, 25, 0, 0, 0, 0, time.Local)
	sessions := MaterializeSessions(schedules, from, to)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions before effective_from, got %d", len(sessions))
	}

	// Within effective range
	from = time.Date(2026, 2, 2, 0, 0, 0, 0, time.Local)
	to = time.Date(2026, 2, 8, 0, 0, 0, 0, time.Local)
	sessions = MaterializeSessions(schedules, from, to)
	if len(sessions) != 1 {
		t.Errorf("expected 1 session within range, got %d", len(sessions))
	}

	// After effective range
	from = time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)
	to = time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	sessions = MaterializeSessions(schedules, from, to)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after effective_until, got %d", len(sessions))
	}
}

func TestDetectConflictsRoom(t *testing.T) {
	schedules := []models.Schedule{
		{ID: "S1", DayOfWeek: "Monday", StartTime: "15:00", EndTime: "16:30", TeacherID: "T01", RoomID: "R01", Subject: "math"},
		{ID: "S2", DayOfWeek: "Monday", StartTime: "16:00", EndTime: "17:30", TeacherID: "T02", RoomID: "R01", Subject: "english"},
	}
	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local)
	sessions := MaterializeSessions(schedules, from, from)
	conflicts := DetectConflicts(sessions)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 room conflict, got %d", len(conflicts))
	}
	if conflicts[0].Type != "room" {
		t.Errorf("expected room conflict, got %s", conflicts[0].Type)
	}
}

func TestDetectConflictsTeacher(t *testing.T) {
	schedules := []models.Schedule{
		{ID: "S1", DayOfWeek: "Monday", StartTime: "15:00", EndTime: "16:30", TeacherID: "T01", RoomID: "R01", Subject: "math"},
		{ID: "S2", DayOfWeek: "Monday", StartTime: "16:00", EndTime: "17:30", TeacherID: "T01", RoomID: "R02", Subject: "science"},
	}
	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local)
	sessions := MaterializeSessions(schedules, from, from)
	conflicts := DetectConflicts(sessions)

	hasTeacher := false
	for _, c := range conflicts {
		if c.Type == "teacher" {
			hasTeacher = true
		}
	}
	if !hasTeacher {
		t.Error("expected teacher conflict")
	}
}

func TestDetectConflictsStudent(t *testing.T) {
	schedules := []models.Schedule{
		{ID: "S1", DayOfWeek: "Monday", StartTime: "15:00", EndTime: "16:30", TeacherID: "T01", RoomID: "R01", StudentIDs: []string{"S001", "S002"}},
		{ID: "S2", DayOfWeek: "Monday", StartTime: "16:00", EndTime: "17:30", TeacherID: "T02", RoomID: "R02", StudentIDs: []string{"S002", "S003"}},
	}
	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local)
	sessions := MaterializeSessions(schedules, from, from)
	conflicts := DetectConflicts(sessions)

	hasStudent := false
	for _, c := range conflicts {
		if c.Type == "student" {
			hasStudent = true
		}
	}
	if !hasStudent {
		t.Error("expected student conflict for S002")
	}
}

func TestNoConflicts(t *testing.T) {
	schedules := []models.Schedule{
		{ID: "S1", DayOfWeek: "Monday", StartTime: "15:00", EndTime: "16:00", TeacherID: "T01", RoomID: "R01"},
		{ID: "S2", DayOfWeek: "Monday", StartTime: "16:00", EndTime: "17:00", TeacherID: "T01", RoomID: "R01"},
	}
	from := time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local)
	sessions := MaterializeSessions(schedules, from, from)
	conflicts := DetectConflicts(sessions)

	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts for adjacent sessions, got %d", len(conflicts))
	}
}
