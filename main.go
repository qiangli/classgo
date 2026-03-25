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
	name := "ClassGo"

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
	ID          int       `json:"id"`
	StudentID   string    `json:"student_id"`
	StudentName string    `json:"student_name"`
	DeviceType  string    `json:"device_type"`
	Timestamp   time.Time `json:"timestamp"`
}

type AdminData struct {
	AppName   string
	PIN       string
	QRDataURI string
	ServerURL string
	Attendees []Attendance
	Count     int
	Date      string
}

type SignInPageData struct {
	AppName   string
	ServerURL string
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./classgo.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS attendance (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id  TEXT NOT NULL,
		student_name TEXT NOT NULL,
		device_type TEXT NOT NULL CHECK(device_type IN ('mobile','kiosk')),
		timestamp   DATETIME DEFAULT (datetime('now','localtime'))
	);
	CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(date(timestamp));
	`
	if _, err := db.Exec(schema); err != nil {
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

func generateQR(content string) string {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		log.Printf("QR generation failed: %v", err)
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func todayAttendees() ([]Attendance, error) {
	rows, err := db.Query(
		"SELECT id, student_id, student_name, device_type, timestamp FROM attendance WHERE date(timestamp) = date('now','localtime') ORDER BY timestamp DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendees []Attendance
	for rows.Next() {
		var a Attendance
		var ts string
		if err := rows.Scan(&a.ID, &a.StudentID, &a.StudentName, &a.DeviceType, &ts); err != nil {
			return nil, err
		}
		a.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		attendees = append(attendees, a)
	}
	return attendees, rows.Err()
}

func handleMobile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := SignInPageData{
		AppName:   appName,
		ServerURL: fmt.Sprintf("http://%s:8080", getLocalIP()),
	}
	tmpl.ExecuteTemplate(w, "mobile.html", data)
}

func handleKiosk(w http.ResponseWriter, r *http.Request) {
	data := SignInPageData{
		AppName:   appName,
		ServerURL: fmt.Sprintf("http://%s:8080", getLocalIP()),
	}
	tmpl.ExecuteTemplate(w, "kiosk.html", data)
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	pin := ensureDailyPIN()
	serverURL := fmt.Sprintf("http://%s:8080", getLocalIP())

	attendees, err := todayAttendees()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Printf("Error fetching attendees: %v", err)
		return
	}

	data := AdminData{
		AppName:   appName,
		PIN:       pin,
		QRDataURI: generateQR(serverURL),
		ServerURL: serverURL,
		Attendees: attendees,
		Count:     len(attendees),
		Date:      time.Now().Format("Monday, January 2, 2006"),
	}
	tmpl.ExecuteTemplate(w, "admin.html", data)
}

func handleSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StudentID   string `json:"student_id"`
		StudentName string `json:"student_name"`
		PIN         string `json:"pin"`
		DeviceType  string `json:"device_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if req.StudentID == "" || req.StudentName == "" || req.PIN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "All fields are required"})
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

	// Check for duplicate sign-in today
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM attendance WHERE student_id = ? AND date(timestamp) = date('now','localtime')",
		req.StudentID,
	).Scan(&count)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
		return
	}
	if count > 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Already signed in today!"})
		return
	}

	_, err = db.Exec(
		"INSERT INTO attendance (student_id, student_name, device_type) VALUES (?, ?, ?)",
		req.StudentID, req.StudentName, req.DeviceType,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to record attendance"})
		log.Printf("Insert error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Welcome, %s!", req.StudentName)})
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
			"SELECT id, student_id, student_name, device_type, timestamp FROM attendance WHERE date(timestamp) BETWEEN ? AND ? ORDER BY timestamp DESC",
			from, to,
		)
	} else {
		rows, err = db.Query(
			"SELECT id, student_id, student_name, device_type, timestamp FROM attendance ORDER BY timestamp DESC",
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
	writer.Write([]string{"ID", "Student ID", "Student Name", "Device Type", "Timestamp"})

	for rows.Next() {
		var id int
		var studentID, studentName, deviceType, timestamp string
		if err := rows.Scan(&id, &studentID, &studentName, &deviceType, &timestamp); err != nil {
			continue
		}
		writer.Write([]string{fmt.Sprintf("%d", id), studentID, studentName, deviceType, timestamp})
	}
	writer.Flush()
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
	mux.HandleFunc("/", handleMobile)
	mux.HandleFunc("/kiosk", handleKiosk)
	mux.HandleFunc("/admin", handleAdmin)
	mux.HandleFunc("/api/signin", handleSignIn)
	mux.HandleFunc("/api/attendees", handleAttendees)
	mux.HandleFunc("/admin/export", handleExport)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	localIP := getLocalIP()
	serverURL := fmt.Sprintf("http://%s:8080", localIP)
	pin := ensureDailyPIN()

	log.Println("=================================")
	log.Printf("  %s Attendance Server", appName)
	log.Println("=================================")
	log.Printf("  Server:  %s", serverURL)
	log.Printf("  Admin:   %s/admin", serverURL)
	log.Printf("  Kiosk:   %s/kiosk", serverURL)
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
