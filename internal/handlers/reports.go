package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"classgo/internal/reports"
)

// HandleReportsPage serves the Reports landing page.
func (a *App) HandleReportsPage(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	data := struct {
		AppName  string
		UserType string
		EntityID string
		UserName string
		Date     string
		Reports  []reports.ReportDef
	}{
		AppName:  a.AppName,
		UserType: sess.UserType,
		EntityID: sess.EntityID,
		Date:     time.Now().Format("Monday, January 2, 2006"),
		Reports:  reports.ReportsForRole(sess.UserType),
	}

	name, _ := a.lookupEntity(sess.EntityID)
	if name == "" {
		name = sess.Username
	}
	data.UserName = name

	a.Tmpl.ExecuteTemplate(w, "reports.html", data)
}

// HandleReportAPI returns report data as JSON.
func (a *App) HandleReportAPI(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	reportType := r.URL.Query().Get("type")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if reportType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Missing type parameter"})
		return
	}

	// Validate role access
	allowed := false
	for _, rd := range reports.ReportsForRole(sess.UserType) {
		if rd.Type == reportType {
			allowed = true
			break
		}
	}
	if !allowed {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "Not authorized for this report"})
		return
	}

	// Default date range
	if from == "" || to == "" {
		now := time.Now()
		switch {
		case containsWord(reportType, "daily"):
			from = now.Format("2006-01-02")
			to = from
		case containsWord(reportType, "weekly", "biweekly"):
			dr := reports.WeekRange(now)
			from = dr.From
			to = dr.To
		case containsWord(reportType, "monthly"):
			dr := reports.MonthRange(now)
			from = dr.From
			to = dr.To
		default:
			dr := reports.WeekRange(now)
			from = dr.From
			to = dr.To
		}
	}

	dr := reports.DateRange{From: from, To: to}

	var result any
	var err error

	switch reportType {
	// Admin reports
	case reports.ReportAdminDailyAttendance:
		result, err = reports.DailyAttendanceRecord(a.DB, from)
	case reports.ReportAdminWeeklyPerf:
		result, err = reports.WeeklyCenterPerformance(a.DB, dr)
	case reports.ReportAdminTeacherWorkload:
		result, err = reports.TeacherWorkloadReport(a.DB, dr)
	case reports.ReportAdminMonthlyDash:
		result, err = reports.MonthlyDashboard(a.DB, dr)
	case reports.ReportAdminEngagement:
		result, err = reports.EngagementScorecard(a.DB, dr)
	case reports.ReportAdminAudit:
		result, err = reports.AuditComplianceLog(a.DB, dr)

	// Teacher reports (scoped to teacher)
	case reports.ReportTeacherWeeklyHours:
		result, err = reports.WeeklyHoursActivity(a.DB, dr, sess.EntityID)
	case reports.ReportTeacherBiweekly:
		biDR := reports.BiweeklyRange(time.Now())
		if from != "" && to != "" {
			biDR = dr
		}
		result, err = reports.BiweeklyStudentSummary(a.DB, biDR, sess.EntityID)
	case reports.ReportTeacherMonthlySummary:
		result, err = reports.MonthlyTeachingSummary(a.DB, dr, sess.EntityID)

	// Parent reports (scoped to parent)
	case reports.ReportParentChildActivity:
		result, err = reports.ChildAttendanceProgress(a.DB, dr, sess.EntityID)

	// Student reports (scoped to student)
	case reports.ReportStudentWeekly:
		result, err = reports.MyWeeklySummary(a.DB, dr, sess.EntityID)
	case reports.ReportStudentMonthly:
		result, err = reports.MyMonthlyProgress(a.DB, dr, sess.EntityID)

	// Timesheet reports
	case reports.ReportAdminStaffTimesheet:
		result, err = reports.AdminStaffTimesheet(a.DB, dr)
	case reports.ReportTeacherTimesheet:
		result, err = reports.StaffTimesheet(a.DB, dr, sess.EntityID)

	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Unknown report type"})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleReportSubscriptions handles CRUD for report subscriptions.
func (a *App) HandleReportSubscriptions(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		subs, err := reports.GetSubscriptions(a.DB, sess.EntityID, sess.UserType)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, subs)

	case http.MethodPost:
		var req struct {
			ReportType string `json:"report_type"`
			Frequency  string `json:"frequency"`
			DayOfWeek  string `json:"day_of_week"`
			Channel    string `json:"channel"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
			return
		}
		if req.DayOfWeek == "" {
			req.DayOfWeek = "friday"
		}
		if req.Channel == "" {
			req.Channel = "email"
		}
		sub := reports.ReportSubscription{
			UserID:     sess.EntityID,
			UserType:   sess.UserType,
			ReportType: req.ReportType,
			Frequency:  req.Frequency,
			DayOfWeek:  req.DayOfWeek,
			Channel:    req.Channel,
		}
		id, err := reports.CreateSubscription(a.DB, sub)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id})

	case http.MethodPut:
		var req struct {
			ID        int    `json:"id"`
			Frequency string `json:"frequency"`
			DayOfWeek string `json:"day_of_week"`
			Channel   string `json:"channel"`
			Active    bool   `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
			return
		}
		if err := reports.UpdateSubscription(a.DB, req.ID, req.Frequency, req.DayOfWeek, req.Channel, req.Active); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid id"})
			return
		}
		if err := reports.DeleteSubscription(a.DB, id, sess.EntityID, sess.UserType); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// HandleReportCatalog returns the list of available reports for the current user's role.
func (a *App) HandleReportCatalog(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, reports.ReportsForRole(sess.UserType))
}

func containsWord(s string, words ...string) bool {
	for _, w := range words {
		if len(s) >= len(w) {
			for i := 0; i <= len(s)-len(w); i++ {
				if s[i:i+len(w)] == w {
					return true
				}
			}
		}
	}
	return false
}
