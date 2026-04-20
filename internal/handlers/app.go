package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"classgo/internal/auth"
	"classgo/internal/database"
	"classgo/internal/memos"
	"classgo/internal/models"
	memosstore "classgo/memos/store"
)

type App struct {
	DB          *sql.DB
	Tmpl        *template.Template
	AppName     string
	DataDir     string
	PinMode     string // "off", "center", "per-student"
	MemosSyncer *memos.Syncer
	MemosStore  *memosstore.Store
	Sessions    *auth.SessionStore
	RateLimiter *RateLimiter

	dailyPIN   string
	pinDate    string
	requirePIN bool
	mu         sync.Mutex
}

// RequireAuth wraps a handler to require authentication (any role). Redirects to login.
func (a *App) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := auth.GetSessionToken(r)
		if token == "" {
			http.Redirect(w, r, auth.LoginPath, http.StatusFound)
			return
		}
		if _, ok := a.Sessions.Get(token); !ok {
			auth.ClearSessionCookie(w)
			http.Redirect(w, r, auth.LoginPath, http.StatusFound)
			return
		}
		next(w, r)
	}
}

// RequireAdmin wraps a handler to require admin role. Redirects to login.
func (a *App) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := auth.GetSessionToken(r)
		if token == "" {
			http.Redirect(w, r, auth.LoginPath, http.StatusFound)
			return
		}
		sess, ok := a.Sessions.Get(token)
		if !ok || sess.Role != "admin" {
			http.Redirect(w, r, auth.LoginPath, http.StatusFound)
			return
		}
		next(w, r)
	}
}

// RequireAdminAPI wraps an API handler to require admin role, returning 401/403 JSON.
func (a *App) RequireAdminAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := auth.GetSessionToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
			return
		}
		sess, ok := a.Sessions.Get(token)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Session expired"})
			return
		}
		if sess.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "Admin access required"})
			return
		}
		next(w, r)
	}
}

// HandleLogin serves the login page and processes login.
func (a *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		token := auth.GetSessionToken(r)
		if token != "" {
			if sess, ok := a.Sessions.Get(token); ok {
				if sess.Role == "admin" {
					http.Redirect(w, r, "/admin", http.StatusFound)
				} else {
					http.Redirect(w, r, "/dashboard", http.StatusFound)
				}
				return
			}
		}
		a.Tmpl.ExecuteTemplate(w, "login.html", map[string]any{
			"AppName": a.AppName,
			"Error":   r.URL.Query().Get("error"),
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// HandleLoginAPI handles login POST as JSON API.
func (a *App) HandleLoginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EntityID  string `json:"entity_id"` // entity ID (e.g., "S001") for user login
		Username  string `json:"username"`  // system username for admin login
		Password  string `json:"password"`
		Action    string `json:"action"`     // "login", "setup", "signup", "admin", "check"
		FirstName string `json:"first_name"` // for signup
		LastName  string `json:"last_name"`  // for signup
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	switch req.Action {
	case "admin":
		// Admin login via OS authentication
		if req.Username == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Username and password required"})
			return
		}
		if err := auth.Authenticate(req.Username, req.Password); err != nil {
			log.Printf("Admin login failed for %q: %v", req.Username, err)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid credentials"})
			return
		}
		token := a.Sessions.Create(req.Username, "admin", "", "")
		auth.SetSessionCookie(w, token)
		log.Printf("Admin login: %s", req.Username)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "role": "admin", "redirect": "/admin"})

	case "check":
		// Check if user has a password set (for first-time detection)
		if req.EntityID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Entity ID required"})
			return
		}
		username := strings.ToLower(req.EntityID)
		ctx := context.Background()
		user, _ := a.MemosStore.GetUser(ctx, &memosstore.FindUser{Username: &username})
		hasPassword := user != nil && user.PasswordHash != ""
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_password": hasPassword})

	case "setup":
		// First-time password setup
		if req.EntityID == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "ID and password required"})
			return
		}
		if len(req.Password) < 4 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Password must be at least 4 characters"})
			return
		}
		name, email := a.lookupEntity(req.EntityID)
		if name == "" {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "User not found"})
			return
		}
		username := strings.ToLower(req.EntityID)
		if _, err := memos.EnsureUser(a.MemosStore, username, name, email, req.Password); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to create account"})
			return
		}
		userType := a.detectUserType(req.EntityID)
		token := a.Sessions.Create(username, "user", userType, req.EntityID)
		auth.SetSessionCookie(w, token)
		log.Printf("User setup + login: %s (%s, %s)", name, username, userType)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "role": "user", "redirect": "/profile"})

	case "login", "":
		// Regular user login via Memos credentials
		if req.EntityID == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "ID and password required"})
			return
		}
		username := strings.ToLower(req.EntityID)
		ctx := context.Background()
		user, err := a.MemosStore.GetUser(ctx, &memosstore.FindUser{Username: &username})
		if err != nil || user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid credentials"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid credentials"})
			return
		}
		userType := a.detectUserType(req.EntityID)
		token := a.Sessions.Create(username, "user", userType, req.EntityID)
		auth.SetSessionCookie(w, token)
		log.Printf("User login: %s (%s, %s)", user.Nickname, username, userType)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "role": "user", "redirect": "/dashboard"})

	case "signup":
		// Signup by name: match student by first+last name, create account
		if req.FirstName == "" || req.LastName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "First name and last name are required"})
			return
		}
		if len(req.Password) < 4 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Password must be at least 4 characters"})
			return
		}
		// Find student by name
		entityID := a.findEntityByName(req.FirstName, req.LastName)
		if entityID == "" {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "No student found with that name. Please check with your administrator."})
			return
		}
		// Check if already has an account
		username := strings.ToLower(entityID)
		ctx := context.Background()
		existingUser, _ := a.MemosStore.GetUser(ctx, &memosstore.FindUser{Username: &username})
		if existingUser != nil && existingUser.PasswordHash != "" {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "Account already exists. Please use Log In instead."})
			return
		}
		// Create account
		name := req.FirstName + " " + req.LastName
		email := ""
		if n, e := a.lookupEntity(entityID); n != "" {
			name = n
			email = e
		}
		if _, err := memos.EnsureUser(a.MemosStore, username, name, email, req.Password); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to create account"})
			return
		}
		userType := a.detectUserType(entityID)
		token := a.Sessions.Create(username, "user", userType, entityID)
		auth.SetSessionCookie(w, token)
		log.Printf("User signup: %s (%s, %s)", name, username, userType)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "role": "user", "redirect": "/profile"})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Unknown action"})
	}
}

// HandleUserSearch searches across students, parents, teachers by id/name/email/phone.
func (a *App) HandleUserSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" || len(q) < 2 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	like := "%" + q + "%"

	type searchResult struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
	}
	var results []searchResult

	for _, tbl := range []struct{ name, typ string }{{"students", "Student"}, {"parents", "Parent"}, {"teachers", "Teacher"}} {
		rows, err := a.DB.Query(
			fmt.Sprintf(`SELECT id, first_name, last_name, COALESCE(email,''), COALESCE(phone,'') FROM %s
			 WHERE deleted = 0 AND (
			   LOWER(id) LIKE LOWER(?) OR
			   LOWER(first_name) LIKE LOWER(?) OR
			   LOWER(last_name) LIKE LOWER(?) OR
			   LOWER(first_name || ' ' || last_name) LIKE LOWER(?) OR
			   LOWER(COALESCE(email,'')) LIKE LOWER(?) OR
			   LOWER(COALESCE(phone,'')) LIKE LOWER(?)
			 ) LIMIT 5`, tbl.name),
			like, like, like, like, like, like,
		)
		if err != nil {
			continue
		}
		for rows.Next() {
			var r searchResult
			r.Type = tbl.typ
			rows.Scan(&r.ID, &r.FirstName, &r.LastName, &r.Email, &r.Phone)
			results = append(results, r)
		}
		rows.Close()
	}

	if results == nil {
		results = []searchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// lookupEntity returns "FirstName LastName" and email for any entity ID across students/parents/teachers.
func (a *App) lookupEntity(entityID string) (name, email string) {
	for _, tbl := range []string{"students", "parents", "teachers"} {
		var fn, ln, em string
		err := a.DB.QueryRow(
			fmt.Sprintf("SELECT first_name, last_name, COALESCE(email,'') FROM %s WHERE id = ?", tbl),
			entityID,
		).Scan(&fn, &ln, &em)
		if err == nil {
			return fn + " " + ln, em
		}
	}
	return "", ""
}

// findEntityByName searches students, parents, teachers by first+last name and returns the entity ID.
func (a *App) findEntityByName(firstName, lastName string) string {
	for _, tbl := range []string{"students", "parents", "teachers"} {
		var id string
		err := a.DB.QueryRow(
			fmt.Sprintf("SELECT id FROM %s WHERE LOWER(first_name) = LOWER(?) AND LOWER(last_name) = LOWER(?) AND deleted = 0", tbl),
			firstName, lastName,
		).Scan(&id)
		if err == nil {
			return id
		}
	}
	return ""
}

// detectUserType determines whether an entity ID belongs to a student, parent, or teacher.
func (a *App) detectUserType(entityID string) string {
	for _, tbl := range []struct{ name, typ string }{{"students", "student"}, {"parents", "parent"}, {"teachers", "teacher"}} {
		var id string
		err := a.DB.QueryRow(fmt.Sprintf("SELECT id FROM %s WHERE id = ?", tbl.name), entityID).Scan(&id)
		if err == nil {
			return tbl.typ
		}
	}
	return ""
}

// GetSession extracts the session from the request, returning nil if not authenticated.
func (a *App) GetSession(r *http.Request) *auth.Session {
	token := auth.GetSessionToken(r)
	if token == "" {
		return nil
	}
	sess, ok := a.Sessions.Get(token)
	if !ok {
		return nil
	}
	return &sess
}

// HandleLogout clears the session and redirects to login.
func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if token != "" {
		a.Sessions.Delete(token)
	}
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, auth.LoginPath, http.StatusFound)
}

// HandleMemosSync triggers a manual Memos sync.
func (a *App) HandleMemosSync(w http.ResponseWriter, r *http.Request) {
	if a.MemosSyncer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "Memos not configured"})
		return
	}

	if r.Method == http.MethodPost {
		// POST triggers attendance summary sync
		if err := a.MemosSyncer.SyncAttendanceSummary(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Attendance summary synced to Memos"})
		return
	}

	// GET triggers full data sync
	if err := a.MemosSyncer.SyncAll(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Data synced to Memos"})
}

func (a *App) EnsureDailyPIN() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if a.pinDate != today {
		a.pinDate = today
		a.dailyPIN = fmt.Sprintf("%04d", rand.Intn(10000))
		log.Printf("New daily PIN for %s: %s", today, a.dailyPIN)
	}
	return a.dailyPIN
}

// SetPIN sets a known PIN for testing.
func (a *App) SetPIN(pin string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dailyPIN = pin
	a.pinDate = time.Now().Format("2006-01-02")
}

func (a *App) RequirePIN() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.requirePIN
}

func (a *App) SetRequirePIN(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requirePIN = v
}

// HandlePINToggle toggles PIN requirement on/off via POST.
func (a *App) HandlePINToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RequirePIN bool `json:"require_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}
	a.SetRequirePIN(req.RequirePIN)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "require_pin": req.RequirePIN})
}

// HandleSettings returns current settings (PIN requirement, etc).
func (a *App) HandleSettings(w http.ResponseWriter, r *http.Request) {
	pinMode := a.PinMode
	if pinMode == "" {
		pinMode = "off"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"require_pin": a.RequirePIN(),
		"pin_mode":    pinMode,
	})
}

// ClientIP extracts the real client IP from the request.
func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := strings.Index(fwd, ","); idx != -1 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// ValidatePIN checks the PIN based on the current mode.
// Per-student override: if a student has require_pin=1, their personal PIN
// is always required regardless of the global PIN mode.
// Returns (needsSetup bool, error string).
func (a *App) ValidatePIN(studentID, pin string) (bool, string) {
	mode := a.PinMode
	if mode == "" {
		mode = "off"
	}

	// Per-student override: if this student requires PIN, enforce personal PIN
	if studentID != "" && mode != "per-student" && database.StudentRequiresPIN(a.DB, studentID) {
		hash, err := database.GetStudentPinHash(a.DB, studentID)
		if err != nil || hash == "" {
			return true, "" // needs setup
		}
		if pin == "" {
			return false, "PIN is required"
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)); err != nil {
			return false, "Invalid PIN"
		}
		return false, ""
	}

	switch mode {
	case "off":
		return false, ""
	case "center":
		if !a.RequirePIN() {
			return false, ""
		}
		if pin == "" {
			return false, "PIN is required"
		}
		if pin != a.EnsureDailyPIN() {
			return false, "Invalid PIN"
		}
		return false, ""
	case "per-student":
		if studentID == "" {
			return false, "Student ID is required for PIN verification"
		}
		hash, err := database.GetStudentPinHash(a.DB, studentID)
		if err != nil || hash == "" {
			// No PIN set yet — needs setup
			return true, ""
		}
		if pin == "" {
			return false, "PIN is required"
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)); err != nil {
			return false, "Invalid PIN"
		}
		return false, ""
	}
	return false, ""
}

// HandleStudentPINSetup allows a student to create their personal PIN.
func (a *App) HandleStudentPINSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		StudentID string `json:"student_id"`
		PIN       string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}
	if req.StudentID == "" || len(req.PIN) < 4 || len(req.PIN) > 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Student ID and 4-6 digit PIN required"})
		return
	}
	// Only allow setup if no PIN exists yet
	existing, _ := database.GetStudentPinHash(a.DB, req.StudentID)
	if existing != "" {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "PIN already set. Ask admin to reset."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.PIN), bcrypt.MinCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to hash PIN"})
		return
	}
	if err := database.SetStudentPinHash(a.DB, req.StudentID, string(hash)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to save PIN"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleStudentPINReset allows admin to reset a student's PIN.
func (a *App) HandleStudentPINReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		StudentID string `json:"student_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}
	if err := database.ResetStudentPin(a.DB, req.StudentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Failed to reset PIN"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandlePINModeChange changes the PIN mode and saves to config.json.
func (a *App) HandlePINModeChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PinMode string `json:"pin_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}
	switch req.PinMode {
	case "off", "center", "per-student":
		a.PinMode = req.PinMode
		// Sync requirePIN for backward compatibility
		a.SetRequirePIN(req.PinMode == "center")
		// Save to config.json
		a.saveConfig()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pin_mode": req.PinMode})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid pin_mode. Use: off, center, per-student"})
	}
}

func (a *App) saveConfig() {
	cfg := models.Config{
		AppName: a.AppName,
		DataDir: a.DataDir,
		PinMode: a.PinMode,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config: %v", err)
		return
	}
	if err := os.WriteFile("config.json", data, 0644); err != nil {
		log.Printf("Failed to write config.json: %v", err)
	}
}

// HandleStudentRequirePIN toggles the require_pin flag for a student.
func (a *App) HandleStudentRequirePIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		StudentID  string `json:"student_id"`
		RequirePIN bool   `json:"require_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.StudentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}
	if err := database.SetStudentRequirePIN(a.DB, req.StudentID, req.RequirePIN); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleAuditFlags returns flagged check-in audit records.
func (a *App) HandleAuditFlags(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().Format("2006-01-02")
	}
	if to == "" {
		to = from
	}
	flags, err := database.GetFlaggedAudits(a.DB, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if flags == nil {
		flags = []models.CheckinAudit{}
	}
	writeJSON(w, http.StatusOK, flags)
}

// HandleAuditDevices returns per-device check-in summary for a date.
func (a *App) HandleAuditDevices(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	summary, err := database.GetDeviceSummary(a.DB, date)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	if summary == nil {
		summary = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, summary)
}

// HandleAuditDismiss dismisses an audit flag.
func (a *App) HandleAuditDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid request"})
		return
	}
	if err := database.DismissAuditFlag(a.DB, req.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandlePINChange allows the admin to set a custom PIN.
func (a *App) HandlePINChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.PIN) != 4 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "PIN must be 4 digits"})
		return
	}
	a.SetPIN(req.PIN)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pin": req.PIN})
}

func GetLocalIP() string {
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

func GetMDNSHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	hostname = strings.TrimSuffix(hostname, ".local")
	hostname = strings.TrimSuffix(hostname, ".")
	return strings.ToLower(hostname) + ".local"
}

func GenerateQR(content string) string {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		log.Printf("QR generation failed: %v", err)
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func NoCache(next http.HandlerFunc) http.HandlerFunc {
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
