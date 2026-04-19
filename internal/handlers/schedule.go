package handlers

import (
	"net/http"
	"time"

	"classgo/internal/datastore"
	"classgo/internal/scheduling"
)

func (a *App) HandleScheduleToday(w http.ResponseWriter, r *http.Request) {
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	sessions := scheduling.TodaySessions(data.Schedules)
	if sessions == nil {
		sessions = []scheduling.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (a *App) HandleScheduleWeek(w http.ResponseWriter, r *http.Request) {
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	sessions := scheduling.WeekSessions(data.Schedules)
	if sessions == nil {
		sessions = []scheduling.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (a *App) HandleScheduleConflicts(w http.ResponseWriter, r *http.Request) {
	data, err := datastore.ReadFromDB(a.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	// Check the next 4 weeks for conflicts
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	to := from.AddDate(0, 0, 28)
	sessions := scheduling.MaterializeSessions(data.Schedules, from, to)
	conflicts := scheduling.DetectConflicts(sessions)
	if conflicts == nil {
		conflicts = []scheduling.Conflict{}
	}
	writeJSON(w, http.StatusOK, conflicts)
}
