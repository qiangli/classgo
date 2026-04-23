package models

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

type Administrator struct {
	Username string `json:"username"`
	Role     string `json:"role"` // "admin" or "superadmin"
}

type CloudSyncConfig struct {
	Enabled            bool   `json:"enabled"`
	Provider           string `json:"provider"`             // rclone backend name, e.g. "drive"
	ServiceAccountFile string `json:"service_account_file"` // relative to DataDir
	FolderID           string `json:"folder_id"`            // remote root folder ID
	Schedule           string `json:"schedule,omitempty"`   // cron expr, default "30 22 * * *"
}

type Config struct {
	AppName        string          `json:"app_name"`
	DataDir        string          `json:"data_dir"`
	PinMode        string          `json:"pin_mode"` // "off", "center", "per-student"
	Port           int             `json:"port,omitempty"`
	DBPath         string          `json:"db_path,omitempty"`
	Administrators []Administrator `json:"administrators,omitempty"`
	CloudSync      CloudSyncConfig `json:"cloud_sync,omitempty"`
}

type CheckinAudit struct {
	ID           int    `json:"id"`
	AttendanceID int    `json:"attendance_id"`
	StudentName  string `json:"student_name"`
	StudentID    string `json:"student_id"`
	DeviceType   string `json:"device_type"`
	ClientIP     string `json:"client_ip"`
	Fingerprint  string `json:"fingerprint"`
	DeviceID     string `json:"device_id"`
	Action       string `json:"action"`
	CreatedAt    string `json:"created_at"`
	Flagged      bool   `json:"flagged"`
	FlagReason   string `json:"flag_reason"`
}

type Student struct {
	ID              string `json:"id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Grade           string `json:"grade"`
	School          string `json:"school"`
	ParentID        string `json:"parent_id"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	Address         string `json:"address"`
	Notes           string `json:"notes"`
	DOB             string `json:"dob"`
	Birthplace      string `json:"birthplace"`
	YearsInUS       string `json:"years_in_us"`
	FirstLanguage   string `json:"first_language"`
	PreviousSchools string `json:"previous_schools"`
	CoursesOutside  string `json:"courses_outside"`
	ProfileStatus   string `json:"profile_status"`
	Active          bool   `json:"active"`
	Deleted         bool   `json:"deleted"`
	DeletedAt       string `json:"deleted_at,omitempty"`
	DeletedBy       string `json:"deleted_by,omitempty"`
	RequirePIN      bool   `json:"require_pin"`
	PersonalPIN     string `json:"personal_pin,omitempty"`
}

type Parent struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Email2    string `json:"email2"`
	Phone2    string `json:"phone2"`
	Address   string `json:"address"`
	Notes     string `json:"notes"`
	Deleted   bool   `json:"deleted"`
	DeletedAt string `json:"deleted_at,omitempty"`
	DeletedBy string `json:"deleted_by,omitempty"`
}

type Teacher struct {
	ID        string   `json:"id"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	Address   string   `json:"address"`
	Subjects  []string `json:"subjects"`
	Active    bool     `json:"active"`
	Deleted   bool     `json:"deleted"`
	DeletedAt string   `json:"deleted_at,omitempty"`
	DeletedBy string   `json:"deleted_by,omitempty"`
}

type Room struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Capacity  int    `json:"capacity"`
	Notes     string `json:"notes"`
	Deleted   bool   `json:"deleted"`
	DeletedAt string `json:"deleted_at,omitempty"`
	DeletedBy string `json:"deleted_by,omitempty"`
}

type Schedule struct {
	ID             string   `json:"id"`
	DayOfWeek      string   `json:"day_of_week"`
	StartTime      string   `json:"start_time"`
	EndTime        string   `json:"end_time"`
	TeacherID      string   `json:"teacher_id"`
	RoomID         string   `json:"room_id"`
	Subject        string   `json:"subject"`
	StudentIDs     []string `json:"student_ids"`
	EffectiveFrom  string   `json:"effective_from"`
	EffectiveUntil string   `json:"effective_until"`
	Deleted        bool     `json:"deleted"`
	DeletedAt      string   `json:"deleted_at,omitempty"`
	DeletedBy      string   `json:"deleted_by,omitempty"`
}

type Attendance struct {
	ID              int        `json:"id"`
	StudentID       string     `json:"student_id"`
	StudentName     string     `json:"student_name"`
	DeviceType      string     `json:"device_type"`
	CheckInTime     time.Time  `json:"-"`
	CheckOutTime    *time.Time `json:"-"`
	CheckInTimeStr  string     `json:"check_in_time"`
	CheckOutTimeStr string     `json:"check_out_time"`
	CheckInRaw      string     `json:"check_in_raw"`
	CheckOutRaw     string     `json:"check_out_raw"`
	Duration        string     `json:"duration"`
	DurationMinutes float64    `json:"duration_minutes"`
	Date            string     `json:"date"`
}

type AdminData struct {
	AppName       string
	PIN           string
	RequirePIN    bool
	QRDataURIIP   template.URL
	QRDataURIMDNS template.URL
	ServerURLIP   string
	ServerURLMDNS string
	Attendees     []Attendance
	Count         int
	Date          string
	IsSuperAdmin  bool
}

type CheckInPageData struct {
	AppName       string
	QRDataURIIP   template.URL
	QRDataURIMDNS template.URL
	ServerURLIP   string
	ServerURLMDNS string
}

// Scope constants for task items.
const (
	ScopeCenter   = 1 // center-wide: applies to all students
	ScopeClass    = 2 // class-scoped: applies to students in a schedule
	ScopePersonal = 3 // personal: applies to one student
)

// Task type constants.
const (
	TaskTypeTodo     = "todo"     // required signoff at checkout (done/undone, must pick)
	TaskTypeTask     = "task"     // optional completion checkbox, won't block checkout
	TaskTypeReminder = "reminder" // info only, no action needed
)

// TaskItem is the unified task/tracker item model.
type TaskItem struct {
	ID          int    `json:"id"`
	Scope       int    `json:"scope"`
	ScheduleID  string `json:"schedule_id"`
	StudentID   string `json:"student_id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Notes       string `json:"notes"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Priority    string `json:"priority"`
	Recurrence  string `json:"recurrence"`
	Category    string `json:"category"`
	Criteria    string `json:"criteria,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	GroupOrder  int    `json:"group_order,omitempty"`
	CreatedBy   string `json:"created_by"`
	OwnerType   string `json:"owner_type"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at"`
	CompletedBy string `json:"completed_by"`
	Active      bool   `json:"active"`
	Deleted     bool   `json:"deleted"`
	DeletedAt   string `json:"deleted_at,omitempty"`
	DeletedBy   string `json:"deleted_by,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// RequiresSignoff returns true if this item type blocks checkout.
func (t TaskItem) RequiresSignoff() bool {
	return t.Type == TaskTypeTodo
}

// ScopeLabel returns a human-readable label for the item's scope.
func (t TaskItem) ScopeLabel() string {
	switch t.Scope {
	case ScopeCenter:
		return "center"
	case ScopeClass:
		return "class"
	case ScopePersonal:
		return "personal"
	}
	return "unknown"
}

// TrackerItem is a backward-compatible alias for center-scoped TaskItem.
type TrackerItem = TaskItem

// StudentTrackerItem is a backward-compatible alias for personal-scoped TaskItem.
type StudentTrackerItem = TaskItem

type TrackerResponse struct {
	ID           int    `json:"id"`
	StudentID    string `json:"student_id"`
	StudentName  string `json:"student_name"`
	ItemType     string `json:"item_type"`
	ItemID       int    `json:"item_id"`
	ItemName     string `json:"item_name"`
	Status       string `json:"status"`
	Notes        string `json:"notes"`
	ResponseDate string `json:"response_date"`
	AttendanceID int    `json:"attendance_id"`
	RespondedAt  string `json:"responded_at"`
	DueDate      string `json:"due_date,omitempty"`
	IsLate       bool   `json:"is_late,omitempty"`
}

type DueItem struct {
	Scope           int    `json:"scope"`
	ItemType        string `json:"item_type"`
	ItemID          int    `json:"item_id"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	Priority        string `json:"priority"`
	Category        string `json:"category"`
	EndDate         string `json:"end_date"`
	Recurrence      string `json:"recurrence"`
	RequiresSignoff bool   `json:"requires_signoff"`
	GroupID         string `json:"group_id,omitempty"`
	GroupOrder      int    `json:"group_order,omitempty"`
	Blocked         bool   `json:"blocked,omitempty"`
}

// TaskGroup holds metadata for a group of related task items.
type TaskGroup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MinRequired  *int   `json:"min_required"` // nil=all required
	EnforceOrder bool   `json:"enforce_order"`
}

type ProgressStats struct {
	StudentID   string  `json:"student_id"`
	StudentName string  `json:"student_name"`
	TotalItems  int     `json:"total_items"`
	DoneCount   int     `json:"done_count"`
	NotDone     int     `json:"not_done_count"`
	Completion  float64 `json:"completion_pct"`
}

type DashboardData struct {
	AppName  string
	UserType string
	EntityID string
	UserName string
	Date     string
}

// ParseTimestamp handles timestamps from modernc.org/sqlite.
// SQLite stores local time via datetime('now','localtime'), but the driver
// returns it as RFC3339 with a "Z" suffix (e.g. "2006-01-02T15:04:05Z").
// The "Z" is misleading — the value is already local time, not UTC.
// We strip the timezone indicator and parse as local time.
func ParseTimestamp(s string) (time.Time, error) {
	s = strings.ReplaceAll(s, "T", " ")
	s = strings.TrimSuffix(s, "Z")
	if idx := strings.LastIndexAny(s, "+-"); idx > 10 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	return time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
}

func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
