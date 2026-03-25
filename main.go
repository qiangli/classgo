package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	_ "modernc.org/sqlite"
)

var (
	db       *sql.DB
	tmpl     *template.Template
	dailyPIN string
	pinDate  string
	mu       sync.Mutex
	appName  string
)

type Config struct {
	AppName string `json:"app_name"`
}

func loadConfig() string {
	name := "LERN"

	// 1. config.json
	if data, err := os.ReadFile("config.json"); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil && cfg.AppName != "" {
			name = cfg.AppName
		}
	}

	// 2. Environment variable
	if env := os.Getenv("APP_NAME"); env != "" {
		name = env
	}

	// 3. Command line flag
	flagName := flag.String("name", "", "Application name")
	flag.Parse()
	if *flagName != "" {
		name = *flagName
	}

	return name
}

type Attendance struct {
	ID              int        `json:"id"`
	StudentName     string     `json:"student_name"`
	DeviceType      string     `json:"device_type"`
	SignInTime      time.Time  `json:"-"`
	SignOutTime     *time.Time `json:"-"`
	SignInTimeStr   string     `json:"sign_in_time"`
	SignOutTimeStr  string     `json:"sign_out_time"`
	SignInRaw       string     `json:"sign_in_raw"`
	SignOutRaw      string     `json:"sign_out_raw"`
	Duration        string     `json:"duration"`
	DurationMinutes float64    `json:"duration_minutes"`
}

type AdminData struct {
	AppName       string
	PIN           string
	QRDataURIIP   template.URL
	QRDataURIMDNS template.URL
	ServerURLIP   string
	ServerURLMDNS string
	Attendees     []Attendance
	Count         int
	Date          string
}

type SignInPageData struct {
	AppName       string
	QRDataURIIP   template.URL
	QRDataURIMDNS template.URL
	ServerURLIP   string
	ServerURLMDNS string
}

func openDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func migrateDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS attendance (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		student_name TEXT NOT NULL,
		device_type  TEXT NOT NULL CHECK(device_type IN ('mobile','kiosk')),
		sign_in_time DATETIME DEFAULT (datetime('now','localtime')),
		sign_out_time DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(date(sign_in_time));
	`
	_, err := db.Exec(schema)
	return err
}

func initDB() {
	var err error
	db, err = openDB("./classgo.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	if err := migrateDB(); err != nil {
		log.Fatal("Failed to initialize database schema:", err)
	}
}

func ensureDailyPIN() string {
	mu.Lock()
	defer mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if pinDate != today {
		pinDate = today
		dailyPIN = fmt.Sprintf("%04d", rand.Intn(10000))
		log.Printf("New daily PIN for %s: %s", today, dailyPIN)
	}
	return dailyPIN
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func getMDNSHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	// Ensure .local suffix for mDNS
	hostname = strings.TrimSuffix(hostname, ".local")
	hostname = strings.TrimSuffix(hostname, ".") // trim trailing dot if any
	return strings.ToLower(hostname) + ".local"
}

func generateQR(content string) string {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		log.Printf("QR generation failed: %v", err)
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// parseTimestamp handles timestamps from modernc.org/sqlite.
// SQLite stores local time via datetime('now','localtime'), but the driver
// returns it as RFC3339 with a "Z" suffix (e.g. "2006-01-02T15:04:05Z").
// The "Z" is misleading — the value is already local time, not UTC.
// We strip the timezone indicator and parse as local time.
func parseTimestamp(s string) (time.Time, error) {
	// Strip timezone suffix and T separator so we can parse as local time
	s = strings.ReplaceAll(s, "T", " ")
	s = strings.TrimSuffix(s, "Z")
	// Remove any explicit offset like +00:00
	if idx := strings.LastIndexAny(s, "+-"); idx > 10 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	return time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
}

func todayAttendees() ([]Attendance, error) {
	rows, err := db.Query(
		"SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance WHERE date(sign_in_time) = date('now','localtime') ORDER BY sign_in_time DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendees []Attendance
	for rows.Next() {
		var a Attendance
		var signIn string
		var signOut sql.NullString
		if err := rows.Scan(&a.ID, &a.StudentName, &a.DeviceType, &signIn, &signOut); err != nil {
			return nil, err
		}
		a.SignInTime, _ = parseTimestamp(signIn)
		a.SignInTimeStr = a.SignInTime.Format("3:04 PM")
		a.SignInRaw = a.SignInTime.Format(time.RFC3339)
		if signOut.Valid {
			t, _ := parseTimestamp(signOut.String)
			a.SignOutTime = &t
			a.SignOutTimeStr = t.Format("3:04 PM")
			a.SignOutRaw = t.Format(time.RFC3339)
			dur := t.Sub(a.SignInTime)
			a.Duration = formatDuration(dur)
			a.DurationMinutes = dur.Minutes()
		}
		attendees = append(attendees, a)
	}
	return attendees, rows.Err()
}

func handleMobile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	ipURL := fmt.Sprintf("http://%s:8080", getLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", getMDNSHostname())
	data := SignInPageData{
		AppName:       appName,
		ServerURLIP:   ipURL,
		ServerURLMDNS: mdnsURL,
	}
	tmpl.ExecuteTemplate(w, "mobile.html", data)
}

func handleKiosk(w http.ResponseWriter, r *http.Request) {
	ipURL := fmt.Sprintf("http://%s:8080", getLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", getMDNSHostname())
	data := SignInPageData{
		AppName:       appName,
		QRDataURIIP:   template.URL(generateQR(ipURL)),
		QRDataURIMDNS: template.URL(generateQR(mdnsURL)),
		ServerURLIP:   ipURL,
		ServerURLMDNS: mdnsURL,
	}
	tmpl.ExecuteTemplate(w, "kiosk.html", data)
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	pin := ensureDailyPIN()
	ipURL := fmt.Sprintf("http://%s:8080", getLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", getMDNSHostname())

	attendees, err := todayAttendees()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Printf("Error fetching attendees: %v", err)
		return
	}

	data := AdminData{
		AppName:       appName,
		PIN:           pin,
		QRDataURIIP:   template.URL(generateQR(ipURL)),
		QRDataURIMDNS: template.URL(generateQR(mdnsURL)),
		ServerURLIP:   ipURL,
		ServerURLMDNS: mdnsURL,
		Attendees:     attendees,
		Count:         len(attendees),
		Date:          time.Now().Format("Monday, January 2, 2006"),
	}
	tmpl.ExecuteTemplate(w, "admin.html", data)
}

func handleSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		PIN         string `json:"pin"`
		DeviceType  string `json:"device_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.StudentName == "" || req.PIN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Name and PIN are required"})
		return
	}

	if req.DeviceType != "mobile" && req.DeviceType != "kiosk" {
		req.DeviceType = "mobile"
	}

	pin := ensureDailyPIN()
	if req.PIN != pin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid PIN"})
		return
	}

	// Check for duplicate sign-in today (no sign-out yet)
	var existingID int
	err := db.QueryRow(
		"SELECT id FROM attendance WHERE student_name = ? AND date(sign_in_time) = date('now','localtime') AND sign_out_time IS NULL LIMIT 1",
		req.StudentName,
	).Scan(&existingID)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Already signed in today!"})
		return
	}

	_, err = db.Exec(
		"INSERT INTO attendance (student_name, device_type) VALUES (?, ?)",
		req.StudentName, req.DeviceType,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to record attendance"})
		log.Printf("Insert error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Welcome, %s!", req.StudentName)})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	studentName := r.URL.Query().Get("student_name")
	if studentName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"signed_in": false, "error": "student_name required"})
		return
	}

	var id int
	var signOutTime sql.NullString
	err := db.QueryRow(
		"SELECT id, sign_out_time FROM attendance WHERE student_name = ? AND date(sign_in_time) = date('now','localtime') ORDER BY sign_in_time DESC LIMIT 1",
		studentName,
	).Scan(&id, &signOutTime)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{"signed_in": false})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"signed_in": false, "error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"signed_in":  true,
		"signed_out": signOutTime.Valid,
	})
}

func handleSignOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentName string `json:"student_name"`
		PIN         string `json:"pin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.StudentName == "" || req.PIN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Name and PIN are required"})
		return
	}

	pin := ensureDailyPIN()
	if req.PIN != pin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid PIN"})
		return
	}

	result, err := db.Exec(
		"UPDATE attendance SET sign_out_time = datetime('now','localtime') WHERE student_name = ? AND date(sign_in_time) = date('now','localtime') AND sign_out_time IS NULL",
		req.StudentName,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "No active sign-in found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Goodbye, %s!", req.StudentName)})
}

func handleAttendees(w http.ResponseWriter, r *http.Request) {
	attendees, err := todayAttendees()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if attendees == nil {
		attendees = []Attendance{}
	}
	writeJSON(w, http.StatusOK, attendees)
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	var rows *sql.Rows
	var err error

	if from != "" && to != "" {
		rows, err = db.Query(
			"SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance WHERE date(sign_in_time) BETWEEN ? AND ? ORDER BY sign_in_time DESC",
			from, to,
		)
	} else {
		rows, err = db.Query(
			"SELECT id, student_name, device_type, sign_in_time, sign_out_time FROM attendance ORDER BY sign_in_time DESC",
		)
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("classgo-export-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Student Name", "Device Type", "Sign In", "Sign Out", "Duration"})

	for rows.Next() {
		var id int
		var studentName, deviceType, signIn string
		var signOut sql.NullString
		if err := rows.Scan(&id, &studentName, &deviceType, &signIn, &signOut); err != nil {
			continue
		}
		inTime, _ := parseTimestamp(signIn)
		signInFmt := inTime.Format("2006-01-02 3:04 PM")
		signOutFmt := ""
		durationStr := ""
		if signOut.Valid {
			outTime, _ := parseTimestamp(signOut.String)
			signOutFmt = outTime.Format("2006-01-02 3:04 PM")
			durationStr = formatDuration(outTime.Sub(inTime))
		}
		writer.Write([]string{fmt.Sprintf("%d", id), studentName, deviceType, signInFmt, signOutFmt, durationStr})
	}
	writer.Flush()
}

func noCache(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	appName = loadConfig()
	initDB()

	var err error
	tmpl, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Failed to parse templates:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", noCache(handleMobile))
	mux.HandleFunc("/kiosk", noCache(handleKiosk))
	mux.HandleFunc("/admin", noCache(handleAdmin))
	mux.HandleFunc("/api/signin", noCache(handleSignIn))
	mux.HandleFunc("/api/signout", noCache(handleSignOut))
	mux.HandleFunc("/api/status", noCache(handleStatus))
	mux.HandleFunc("/api/attendees", noCache(handleAttendees))
	mux.HandleFunc("/admin/export", noCache(handleExport))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	ipURL := fmt.Sprintf("http://%s:8080", getLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", getMDNSHostname())
	pin := ensureDailyPIN()

	log.Println("=================================")
	log.Printf("  %s Attendance Server", appName)
	log.Println("=================================")
	log.Printf("  Server:  %s", mdnsURL)
	log.Printf("           %s", ipURL)
	log.Printf("  Admin:   %s/admin", mdnsURL)
	log.Printf("  Kiosk:   %s/kiosk", mdnsURL)
	log.Printf("  PIN:     %s", pin)
	log.Println("=================================")

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		db.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal("Server error:", err)
	}
}
