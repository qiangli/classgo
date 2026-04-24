package reports

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// RunDailyAttendance generates and archives the A1 daily attendance report.
func RunDailyAttendance(db *sql.DB, dataDir string) {
	today := time.Now().Format("2006-01-02")
	report, err := DailyAttendanceRecord(db, today)
	if err != nil {
		log.Printf("Report: daily attendance failed: %v", err)
		return
	}

	// Archive as XLSX
	outDir := filepath.Join(dataDir, "reports", "admin")
	os.MkdirAll(outDir, 0755)
	outPath := filepath.Join(outDir, "attendance-"+today+".xlsx")
	if err := writeDailyAttendanceXLSX(report, outPath); err != nil {
		log.Printf("Report: daily attendance XLSX failed: %v", err)
		return
	}
	log.Printf("Report: daily attendance archived → %s", outPath)
}

// RunWeeklyAudit generates and archives the A6 audit report.
func RunWeeklyAudit(db *sql.DB, dataDir string) {
	dr := WeekRange(time.Now())
	report, err := AuditComplianceLog(db, dr)
	if err != nil {
		log.Printf("Report: weekly audit failed: %v", err)
		return
	}

	outDir := filepath.Join(dataDir, "reports", "admin")
	os.MkdirAll(outDir, 0755)
	_, isoWeek := time.Now().ISOWeek()
	outPath := filepath.Join(outDir, "audit-"+time.Now().Format("2006")+"-W"+itoa(isoWeek)+".xlsx")
	if err := writeAuditXLSX(report, outPath); err != nil {
		log.Printf("Report: weekly audit XLSX failed: %v", err)
		return
	}
	log.Printf("Report: weekly audit archived → %s", outPath)
}

// RunMonthlyDashboard generates and archives the A4 monthly report.
func RunMonthlyDashboard(db *sql.DB, dataDir string) {
	// Report for previous month
	prev := time.Now().AddDate(0, -1, 0)
	dr := MonthRange(prev)
	report, err := MonthlyDashboard(db, dr)
	if err != nil {
		log.Printf("Report: monthly dashboard failed: %v", err)
		return
	}

	outDir := filepath.Join(dataDir, "reports", "admin")
	os.MkdirAll(outDir, 0755)
	outPath := filepath.Join(outDir, "monthly-"+prev.Format("2006-01")+".xlsx")
	if err := writeMonthlyXLSX(report, outPath); err != nil {
		log.Printf("Report: monthly dashboard XLSX failed: %v", err)
		return
	}
	log.Printf("Report: monthly dashboard archived → %s", outPath)
}

// ProcessSubscriptions checks active subscriptions and runs any that are due today.
func ProcessSubscriptions(db *sql.DB, dataDir string) {
	subs, err := GetActiveSubscriptions(db)
	if err != nil {
		log.Printf("Report: subscription processor failed: %v", err)
		return
	}

	now := time.Now()
	todayDow := strings.ToLower(now.Weekday().String())
	_, isoWeek := now.ISOWeek()

	for _, sub := range subs {
		if !isDue(sub, todayDow, isoWeek, now) {
			continue
		}

		log.Printf("Report: processing subscription #%d (%s for %s %s)", sub.ID, sub.ReportType, sub.UserType, sub.UserID)

		// Generate the report based on type and frequency
		dr := dateRangeForFrequency(sub.Frequency, now)

		switch sub.ReportType {
		case ReportParentChildActivity:
			report, err := ChildAttendanceProgress(db, dr, sub.UserID)
			if err != nil {
				log.Printf("Report: subscription #%d failed: %v", sub.ID, err)
				continue
			}
			// Archive
			outDir := filepath.Join(dataDir, "reports", "parent")
			os.MkdirAll(outDir, 0755)
			outPath := filepath.Join(outDir, "child-activity-"+sub.UserID+"-"+now.Format("2006-01-02")+".xlsx")
			writeParentXLSX(report, outPath)
			log.Printf("Report: parent subscription archived → %s", outPath)
		}
		// Add more report types as they become schedulable
	}
}

// isDue checks if a subscription should fire today based on its frequency and day_of_week.
func isDue(sub ReportSubscription, todayDow string, isoWeek int, now time.Time) bool {
	if sub.DayOfWeek != "" && strings.ToLower(sub.DayOfWeek) != todayDow {
		return false
	}

	switch sub.Frequency {
	case "weekly":
		return true // day_of_week already matched
	case "biweekly":
		return isoWeek%2 == 0 // every even ISO week
	case "monthly":
		return now.Day() == 1 // first of month
	}
	return false
}

// dateRangeForFrequency returns an appropriate date range based on subscription frequency.
func dateRangeForFrequency(freq string, now time.Time) DateRange {
	switch freq {
	case "weekly":
		return WeekRange(now.AddDate(0, 0, -1)) // previous week
	case "biweekly":
		return BiweeklyRange(now)
	case "monthly":
		return MonthRange(now.AddDate(0, -1, 0)) // previous month
	}
	return WeekRange(now)
}

// --- XLSX writers ---

func writeDailyAttendanceXLSX(report *AdminDailyReport, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Attendance"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"Student ID", "Student Name", "Device", "Check In", "Check Out", "Duration (min)"}
	for i, h := range headers {
		f.SetCellValue(sheet, cellRef(i+1, 1), h)
	}

	for i, r := range report.Records {
		row := i + 2
		f.SetCellValue(sheet, cellRef(1, row), r.StudentID)
		f.SetCellValue(sheet, cellRef(2, row), r.StudentName)
		f.SetCellValue(sheet, cellRef(3, row), r.DeviceType)
		f.SetCellValue(sheet, cellRef(4, row), r.CheckIn)
		f.SetCellValue(sheet, cellRef(5, row), r.CheckOut)
		f.SetCellValue(sheet, cellRef(6, row), r.DurationMin)
	}

	// Summary sheet
	summary := "Summary"
	f.NewSheet(summary)
	f.SetCellValue(summary, "A1", "Date")
	f.SetCellValue(summary, "B1", report.Date)
	f.SetCellValue(summary, "A2", "Total Check-ins")
	f.SetCellValue(summary, "B2", report.TotalCheckIns)
	f.SetCellValue(summary, "A3", "Unique Students")
	f.SetCellValue(summary, "B3", report.UniqueStudents)
	f.SetCellValue(summary, "A4", "Open Sessions")
	f.SetCellValue(summary, "B4", report.OpenSessions)
	f.SetCellValue(summary, "A5", "Avg Duration (min)")
	f.SetCellValue(summary, "B5", report.AvgDurationMins)
	f.SetCellValue(summary, "A6", "Fraud Flags")
	f.SetCellValue(summary, "B6", report.FraudFlags)

	return f.SaveAs(path)
}

func writeAuditXLSX(report *AdminAuditReport, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Flagged Events"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"Student Name", "Student ID", "Device", "IP", "Reason", "Time"}
	for i, h := range headers {
		f.SetCellValue(sheet, cellRef(i+1, 1), h)
	}
	for i, e := range report.FlaggedEvents {
		row := i + 2
		f.SetCellValue(sheet, cellRef(1, row), e.StudentName)
		f.SetCellValue(sheet, cellRef(2, row), e.StudentID)
		f.SetCellValue(sheet, cellRef(3, row), e.DeviceType)
		f.SetCellValue(sheet, cellRef(4, row), e.ClientIP)
		f.SetCellValue(sheet, cellRef(5, row), e.FlagReason)
		f.SetCellValue(sheet, cellRef(6, row), e.CreatedAt)
	}

	return f.SaveAs(path)
}

func writeMonthlyXLSX(report *AdminMonthlyReport, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Summary"
	f.SetSheetName("Sheet1", sheet)
	f.SetCellValue(sheet, "A1", "Period")
	f.SetCellValue(sheet, "B1", report.DateRange.From+" to "+report.DateRange.To)
	f.SetCellValue(sheet, "A2", "Total Check-ins")
	f.SetCellValue(sheet, "B2", report.TotalCheckIns)
	f.SetCellValue(sheet, "A3", "Unique Students")
	f.SetCellValue(sheet, "B3", report.UniqueStudents)
	f.SetCellValue(sheet, "A4", "Avg Daily Attendance")
	f.SetCellValue(sheet, "B4", report.AvgDailyAttend)
	f.SetCellValue(sheet, "A5", "New Students")
	f.SetCellValue(sheet, "B5", report.NewStudents)
	f.SetCellValue(sheet, "A6", "Inactive Students")
	f.SetCellValue(sheet, "B6", report.InactiveStudents)
	f.SetCellValue(sheet, "A7", "Completion Rate (%)")
	f.SetCellValue(sheet, "B7", report.CompletionRate)

	// Weekly breakdown sheet
	weeks := "Weekly Breakdown"
	f.NewSheet(weeks)
	f.SetCellValue(weeks, "A1", "Week")
	f.SetCellValue(weeks, "B1", "Check-ins")
	f.SetCellValue(weeks, "C1", "Unique Students")
	for i, w := range report.WeeklyBreakdown {
		row := i + 2
		f.SetCellValue(weeks, cellRef(1, row), w.WeekLabel)
		f.SetCellValue(weeks, cellRef(2, row), w.CheckIns)
		f.SetCellValue(weeks, cellRef(3, row), w.Unique)
	}

	return f.SaveAs(path)
}

func writeParentXLSX(report *ParentChildReport, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	for i, child := range report.Children {
		sheet := child.StudentName
		if i == 0 {
			f.SetSheetName("Sheet1", sheet)
		} else {
			f.NewSheet(sheet)
		}

		f.SetCellValue(sheet, "A1", "Student")
		f.SetCellValue(sheet, "B1", child.StudentName)
		f.SetCellValue(sheet, "A2", "Days Attended")
		f.SetCellValue(sheet, "B2", child.DaysAttended)
		f.SetCellValue(sheet, "A3", "Total Hours")
		f.SetCellValue(sheet, "B3", child.TotalHours)
		f.SetCellValue(sheet, "A4", "Completion (%)")
		f.SetCellValue(sheet, "B4", child.CompletionPct)
		f.SetCellValue(sheet, "A5", "Engagement")
		f.SetCellValue(sheet, "B5", child.EngagementLevel)

		// Daily detail
		f.SetCellValue(sheet, "A7", "Date")
		f.SetCellValue(sheet, "B7", "Check In")
		f.SetCellValue(sheet, "C7", "Check Out")
		f.SetCellValue(sheet, "D7", "Duration")
		for j, d := range child.DailyDetail {
			row := j + 8
			f.SetCellValue(sheet, cellRef(1, row), d.Date)
			f.SetCellValue(sheet, cellRef(2, row), d.CheckIn)
			f.SetCellValue(sheet, cellRef(3, row), d.CheckOut)
			f.SetCellValue(sheet, cellRef(4, row), d.Duration)
		}
	}

	return f.SaveAs(path)
}

// cellRef returns an Excel cell reference like "A1", "B2", etc.
func cellRef(col, row int) string {
	letter := string(rune('A' + col - 1))
	if col > 26 {
		letter = string(rune('A'+col/26-1)) + string(rune('A'+col%26-1))
	}
	return letter + itoa(row)
}
