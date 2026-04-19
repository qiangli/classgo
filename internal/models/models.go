package models

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

type Config struct {
	AppName string `json:"app_name"`
	DataDir string `json:"data_dir"`
}

type Student struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Grade     string `json:"grade"`
	School    string `json:"school"`
	ParentID  string `json:"parent_id"`
	Notes     string `json:"notes"`
	Active    bool   `json:"active"`
}

type Parent struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Notes     string `json:"notes"`
}

type Teacher struct {
	ID        string   `json:"id"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	Subjects  []string `json:"subjects"`
	Active    bool     `json:"active"`
}

type Room struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
	Notes    string `json:"notes"`
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
}

type Attendance struct {
	ID              int        `json:"id"`
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
}

type CheckInPageData struct {
	AppName       string
	QRDataURIIP   template.URL
	QRDataURIMDNS template.URL
	ServerURLIP   string
	ServerURLMDNS string
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
