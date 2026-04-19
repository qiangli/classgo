package scheduling

import (
	"fmt"
	"sort"
)

// Conflict represents a scheduling conflict between two sessions.
type Conflict struct {
	Type     string  `json:"type"`     // "room", "teacher", "student"
	Session1 Session `json:"session1"`
	Session2 Session `json:"session2"`
	Detail   string  `json:"detail"`
}

// DetectConflicts checks sessions for room, teacher, and student overlaps.
func DetectConflicts(sessions []Session) []Conflict {
	var conflicts []Conflict

	// Group by date
	byDate := make(map[string][]Session)
	for _, s := range sessions {
		byDate[s.DateStr] = append(byDate[s.DateStr], s)
	}

	for _, daySessions := range byDate {
		// Sort by start time
		sort.Slice(daySessions, func(i, j int) bool {
			return daySessions[i].StartTime < daySessions[j].StartTime
		})

		// Pairwise comparison for overlaps
		for i := 0; i < len(daySessions); i++ {
			for j := i + 1; j < len(daySessions); j++ {
				s1, s2 := daySessions[i], daySessions[j]
				if !timesOverlap(s1.StartTime, s1.EndTime, s2.StartTime, s2.EndTime) {
					continue
				}

				// Room conflict
				if s1.RoomID != "" && s1.RoomID == s2.RoomID {
					conflicts = append(conflicts, Conflict{
						Type:     "room",
						Session1: s1,
						Session2: s2,
						Detail:   fmt.Sprintf("Room %s double-booked on %s: %s-%s (%s) vs %s-%s (%s)", s1.RoomID, s1.DateStr, s1.StartTime, s1.EndTime, s1.Subject, s2.StartTime, s2.EndTime, s2.Subject),
					})
				}

				// Teacher conflict
				if s1.TeacherID != "" && s1.TeacherID == s2.TeacherID {
					conflicts = append(conflicts, Conflict{
						Type:     "teacher",
						Session1: s1,
						Session2: s2,
						Detail:   fmt.Sprintf("Teacher %s double-booked on %s: %s-%s (%s) vs %s-%s (%s)", s1.TeacherID, s1.DateStr, s1.StartTime, s1.EndTime, s1.Subject, s2.StartTime, s2.EndTime, s2.Subject),
					})
				}

				// Student conflicts
				for _, sid1 := range s1.StudentIDs {
					for _, sid2 := range s2.StudentIDs {
						if sid1 == sid2 {
							conflicts = append(conflicts, Conflict{
								Type:     "student",
								Session1: s1,
								Session2: s2,
								Detail:   fmt.Sprintf("Student %s double-booked on %s: %s-%s (%s) vs %s-%s (%s)", sid1, s1.DateStr, s1.StartTime, s1.EndTime, s1.Subject, s2.StartTime, s2.EndTime, s2.Subject),
							})
						}
					}
				}
			}
		}
	}

	return conflicts
}

// timesOverlap checks if two time ranges (HH:MM format) overlap.
func timesOverlap(start1, end1, start2, end2 string) bool {
	// Two ranges overlap if one starts before the other ends
	return start1 < end2 && start2 < end1
}
