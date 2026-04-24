package reports

import (
	"time"
)

// DateRange represents a date range for report queries.
type DateRange struct {
	From string // YYYY-MM-DD
	To   string // YYYY-MM-DD
}

// WeekRange returns a DateRange for the ISO week containing the given date.
func WeekRange(t time.Time) DateRange {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	monday := t.AddDate(0, 0, -(weekday - 1))
	sunday := monday.AddDate(0, 0, 6)
	return DateRange{
		From: monday.Format("2006-01-02"),
		To:   sunday.Format("2006-01-02"),
	}
}

// MonthRange returns a DateRange for the full month containing the given date.
func MonthRange(t time.Time) DateRange {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	last := first.AddDate(0, 1, -1)
	return DateRange{
		From: first.Format("2006-01-02"),
		To:   last.Format("2006-01-02"),
	}
}

// BiweeklyRange returns a DateRange for the two weeks ending on the given date.
func BiweeklyRange(t time.Time) DateRange {
	return DateRange{
		From: t.AddDate(0, 0, -13).Format("2006-01-02"),
		To:   t.Format("2006-01-02"),
	}
}

// Report types identify each report for API and subscription routing.
const (
	// Admin reports
	ReportAdminDailyAttendance = "admin-daily-attendance"
	ReportAdminWeeklyPerf      = "admin-weekly-performance"
	ReportAdminTeacherWorkload = "admin-teacher-workload"
	ReportAdminMonthlyDash     = "admin-monthly-dashboard"
	ReportAdminEngagement      = "admin-engagement-scorecard"
	ReportAdminAudit           = "admin-audit-compliance"

	// Teacher reports
	ReportTeacherWeeklyHours    = "teacher-weekly-hours"
	ReportTeacherBiweekly       = "teacher-biweekly-summary"
	ReportTeacherMonthlySummary = "teacher-monthly-summary"

	// Parent reports
	ReportParentChildActivity = "parent-child-activity"

	// Student reports
	ReportStudentWeekly  = "student-weekly-summary"
	ReportStudentMonthly = "student-monthly-progress"
)

// ReportDef describes a report available in the system.
type ReportDef struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Roles       []string `json:"roles"` // which user types can view this
	Schedulable bool     `json:"schedulable"`
}

// AllReports returns the full catalog of available reports.
func AllReports() []ReportDef {
	return []ReportDef{
		// Admin
		{ReportAdminDailyAttendance, "Daily Attendance Record", "Full attendance log with check-in/out times, duration, open sessions, device breakdown", []string{"admin"}, false},
		{ReportAdminWeeklyPerf, "Weekly Center Performance", "Attendance trends, no-shows, new students, week-over-week comparison", []string{"admin"}, false},
		{ReportAdminTeacherWorkload, "Teacher Workload & Utilization", "Per-teacher hours, classes, students, room utilization", []string{"admin"}, false},
		{ReportAdminMonthlyDash, "Monthly Center Dashboard", "Enrollment growth, completion rates, utilization, profile gaps", []string{"admin"}, false},
		{ReportAdminEngagement, "Student Engagement Scorecard", "Per-student composite score, at-risk identification, trends", []string{"admin"}, false},
		{ReportAdminAudit, "Audit & Compliance Log", "Fraud flags, device analysis, repeat offenders", []string{"admin"}, false},

		// Teacher
		{ReportTeacherWeeklyHours, "Weekly Hours & Activity", "Total hours taught, classes conducted, students served", []string{"teacher"}, false},
		{ReportTeacherBiweekly, "Biweekly Student Summary", "Per-student attendance and completion over 2 weeks", []string{"teacher"}, false},
		{ReportTeacherMonthlySummary, "Monthly Teaching Summary", "Monthly hours with per-student breakdown", []string{"teacher"}, false},

		// Parent
		{ReportParentChildActivity, "Child Attendance & Progress", "Per-child attendance, hours, task completion, upcoming tasks", []string{"parent"}, true},

		// Student
		{ReportStudentWeekly, "My Weekly Summary", "Days attended, hours, completion, streak", []string{"student"}, false},
		{ReportStudentMonthly, "My Monthly Progress", "Monthly trends, category breakdown, engagement level", []string{"student"}, false},
	}
}

// ReportsForRole returns only the reports visible to a given user type.
func ReportsForRole(userType string) []ReportDef {
	role := userType
	if role == "" {
		role = "admin"
	}
	var result []ReportDef
	for _, r := range AllReports() {
		for _, allowed := range r.Roles {
			if allowed == role {
				result = append(result, r)
				break
			}
		}
	}
	return result
}

// --- Admin report result types ---

type AdminDailyReport struct {
	Date            string           `json:"date"`
	TotalCheckIns   int              `json:"total_checkins"`
	UniqueStudents  int              `json:"unique_students"`
	TotalCheckOuts  int              `json:"total_checkouts"`
	OpenSessions    int              `json:"open_sessions"`
	AvgDurationMins float64          `json:"avg_duration_mins"`
	DeviceBreakdown map[string]int   `json:"device_breakdown"`
	FraudFlags      int              `json:"fraud_flags"`
	Records         []AttendanceRow  `json:"records"`
	OpenSessionList []AttendanceRow  `json:"open_session_list"`
	PrevWeekCompare *PrevWeekCompare `json:"prev_week_compare,omitempty"`
}

type AttendanceRow struct {
	StudentID   string  `json:"student_id"`
	StudentName string  `json:"student_name"`
	DeviceType  string  `json:"device_type"`
	CheckIn     string  `json:"check_in"`
	CheckOut    string  `json:"check_out"`
	Duration    string  `json:"duration"`
	DurationMin float64 `json:"duration_min"`
}

type PrevWeekCompare struct {
	PrevCheckIns  int `json:"prev_checkins"`
	PrevUnique    int `json:"prev_unique"`
	DeltaCheckIns int `json:"delta_checkins"`
	DeltaUnique   int `json:"delta_unique"`
}

type AdminWeeklyReport struct {
	DateRange       DateRange        `json:"date_range"`
	TotalCheckIns   int              `json:"total_checkins"`
	UniqueStudents  int              `json:"unique_students"`
	AvgDailyAttend  float64          `json:"avg_daily_attendance"`
	ByDay           []DayBreakdown   `json:"by_day"`
	NoShows         []StudentBasic   `json:"no_shows"`
	NewStudents     []StudentBasic   `json:"new_students"`
	ConsistentCount int              `json:"consistent_count"`
	SporadicCount   int              `json:"sporadic_count"`
	PrevWeek        *PrevWeekCompare `json:"prev_week_compare,omitempty"`
	CompletionRate  float64          `json:"completion_rate"`
	FraudFlagCount  int              `json:"fraud_flag_count"`
}

type DayBreakdown struct {
	Date     string  `json:"date"`
	DayName  string  `json:"day_name"`
	CheckIns int     `json:"checkins"`
	AvgMins  float64 `json:"avg_mins"`
}

type StudentBasic struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AdminTeacherWorkload struct {
	DateRange DateRange            `json:"date_range"`
	Teachers  []TeacherWorkloadRow `json:"teachers"`
	Rooms     []RoomUtilizationRow `json:"rooms"`
}

type TeacherWorkloadRow struct {
	TeacherID     string  `json:"teacher_id"`
	TeacherName   string  `json:"teacher_name"`
	ClassCount    int     `json:"class_count"`
	TotalHours    float64 `json:"total_hours"`
	EnrolledCount int     `json:"enrolled_count"`
	AttendedCount int     `json:"attended_count"`
	TasksCreated  int     `json:"tasks_created"`
}

type RoomUtilizationRow struct {
	RoomID      string  `json:"room_id"`
	RoomName    string  `json:"room_name"`
	Capacity    int     `json:"capacity"`
	AvgEnrolled float64 `json:"avg_enrolled"`
	Utilization float64 `json:"utilization_pct"`
}

type AdminMonthlyReport struct {
	DateRange        DateRange         `json:"date_range"`
	TotalCheckIns    int               `json:"total_checkins"`
	UniqueStudents   int               `json:"unique_students"`
	AvgDailyAttend   float64           `json:"avg_daily_attendance"`
	WeeklyBreakdown  []WeekBreakdown   `json:"weekly_breakdown"`
	NewStudents      int               `json:"new_students"`
	InactiveStudents int               `json:"inactive_students"`
	CompletionRate   float64           `json:"completion_rate"`
	ProfileGaps      ProfileGapSummary `json:"profile_gaps"`
	FraudFlagCount   int               `json:"fraud_flag_count"`
}

type WeekBreakdown struct {
	WeekLabel string `json:"week_label"`
	From      string `json:"from"`
	To        string `json:"to"`
	CheckIns  int    `json:"checkins"`
	Unique    int    `json:"unique_students"`
}

type ProfileGapSummary struct {
	MissingGrade   int `json:"missing_grade"`
	MissingParent  int `json:"missing_parent"`
	MissingDOB     int `json:"missing_dob"`
	ParentsNoEmail int `json:"parents_no_email"`
	TotalStudents  int `json:"total_students"`
}

type EngagementScore struct {
	StudentID       string  `json:"student_id"`
	StudentName     string  `json:"student_name"`
	Grade           string  `json:"grade"`
	School          string  `json:"school"`
	AttendanceScore float64 `json:"attendance_score"`
	CompletionScore float64 `json:"completion_score"`
	DurationScore   float64 `json:"duration_score"`
	CompositeScore  float64 `json:"composite_score"`
	AtRisk          bool    `json:"at_risk"`
}

type AdminEngagementReport struct {
	DateRange   DateRange         `json:"date_range"`
	Students    []EngagementScore `json:"students"`
	AtRiskCount int               `json:"at_risk_count"`
	AvgScore    float64           `json:"avg_score"`
}

type AdminAuditReport struct {
	DateRange       DateRange        `json:"date_range"`
	TotalFlags      int              `json:"total_flags"`
	FlaggedEvents   []AuditEvent     `json:"flagged_events"`
	RepeatOffenders []RepeatOffender `json:"repeat_offenders"`
}

type AuditEvent struct {
	StudentName string `json:"student_name"`
	StudentID   string `json:"student_id"`
	DeviceType  string `json:"device_type"`
	ClientIP    string `json:"client_ip"`
	FlagReason  string `json:"flag_reason"`
	CreatedAt   string `json:"created_at"`
}

type RepeatOffender struct {
	StudentName string `json:"student_name"`
	StudentID   string `json:"student_id"`
	FlagCount   int    `json:"flag_count"`
}

// --- Teacher report result types ---

type TeacherWeeklyHoursReport struct {
	DateRange      DateRange         `json:"date_range"`
	TeacherID      string            `json:"teacher_id"`
	TeacherName    string            `json:"teacher_name"`
	TotalHours     float64           `json:"total_hours"`
	ClassCount     int               `json:"class_count"`
	UniqueStudents int               `json:"unique_students"`
	Classes        []ClassSummaryRow `json:"classes"`
}

type ClassSummaryRow struct {
	ScheduleID string   `json:"schedule_id"`
	DayOfWeek  string   `json:"day_of_week"`
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	Subject    string   `json:"subject"`
	RoomName   string   `json:"room_name"`
	Enrolled   int      `json:"enrolled"`
	Attended   int      `json:"attended"`
	Absent     []string `json:"absent"`
}

type TeacherBiweeklyReport struct {
	DateRange   DateRange                `json:"date_range"`
	TeacherID   string                   `json:"teacher_id"`
	TeacherName string                   `json:"teacher_name"`
	Students    []TeacherStudentProgress `json:"students"`
	AtRiskCount int                      `json:"at_risk_count"`
}

type TeacherStudentProgress struct {
	StudentID       string  `json:"student_id"`
	StudentName     string  `json:"student_name"`
	DaysAttended    int     `json:"days_attended"`
	TotalHours      float64 `json:"total_hours"`
	CompletionPct   float64 `json:"completion_pct"`
	Trend           string  `json:"trend"` // "improving", "declining", "flat"
	AtRisk          bool    `json:"at_risk"`
	IncompleteItems int     `json:"incomplete_items"`
}

type TeacherMonthlyReport struct {
	DateRange          DateRange                `json:"date_range"`
	TeacherID          string                   `json:"teacher_id"`
	TeacherName        string                   `json:"teacher_name"`
	TotalHours         float64                  `json:"total_hours"`
	WeeklyHours        []WeekHoursRow           `json:"weekly_hours"`
	Students           []TeacherStudentProgress `json:"students"`
	ClassAvgAttendance float64                  `json:"class_avg_attendance"`
	ClassAvgCompletion float64                  `json:"class_avg_completion"`
}

type WeekHoursRow struct {
	WeekLabel string  `json:"week_label"`
	Hours     float64 `json:"hours"`
}

// --- Parent report result types ---

type ParentChildReport struct {
	DateRange  DateRange      `json:"date_range"`
	ParentID   string         `json:"parent_id"`
	ParentName string         `json:"parent_name"`
	Children   []ChildSummary `json:"children"`
}

type ChildSummary struct {
	StudentID       string           `json:"student_id"`
	StudentName     string           `json:"student_name"`
	DaysAttended    int              `json:"days_attended"`
	TotalHours      float64          `json:"total_hours"`
	CompletionPct   float64          `json:"completion_pct"`
	EngagementLevel string           `json:"engagement_level"` // "Excellent", "Good", "Needs Attention"
	DailyDetail     []ChildDayDetail `json:"daily_detail"`
	IncompleteItems []IncompleteItem `json:"incomplete_items"`
	UpcomingTasks   []UpcomingTask   `json:"upcoming_tasks"`
}

type ChildDayDetail struct {
	Date     string `json:"date"`
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Duration string `json:"duration"`
}

type IncompleteItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	IsLate bool   `json:"is_late"`
}

type UpcomingTask struct {
	Name     string `json:"name"`
	DueDate  string `json:"due_date"`
	Priority string `json:"priority"`
}

// --- Student report result types ---

type StudentWeeklyReport struct {
	DateRange      DateRange `json:"date_range"`
	StudentID      string    `json:"student_id"`
	StudentName    string    `json:"student_name"`
	DaysAttended   int       `json:"days_attended"`
	TotalHours     float64   `json:"total_hours"`
	TasksCompleted int       `json:"tasks_completed"`
	TasksTotal     int       `json:"tasks_total"`
	CompletionPct  float64   `json:"completion_pct"`
	LateCount      int       `json:"late_count"`
	Streak         int       `json:"streak"`
}

type StudentMonthlyReport struct {
	DateRange         DateRange            `json:"date_range"`
	StudentID         string               `json:"student_id"`
	StudentName       string               `json:"student_name"`
	DaysAttended      int                  `json:"days_attended"`
	TotalHours        float64              `json:"total_hours"`
	CompletionPct     float64              `json:"completion_pct"`
	EngagementLevel   string               `json:"engagement_level"`
	CategoryBreakdown []CategoryCompletion `json:"category_breakdown"`
	WeeklyTrend       []WeekCompletionRow  `json:"weekly_trend"`
}

type CategoryCompletion struct {
	Category      string  `json:"category"`
	CompletionPct float64 `json:"completion_pct"`
}

type WeekCompletionRow struct {
	WeekLabel     string  `json:"week_label"`
	CompletionPct float64 `json:"completion_pct"`
}

// --- Subscription types ---

type ReportSubscription struct {
	ID         int    `json:"id"`
	UserID     string `json:"user_id"`
	UserType   string `json:"user_type"`
	ReportType string `json:"report_type"`
	Frequency  string `json:"frequency"`
	DayOfWeek  string `json:"day_of_week"`
	Channel    string `json:"channel"`
	Active     bool   `json:"active"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
