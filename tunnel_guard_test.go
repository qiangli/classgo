package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"classgo/internal/handlers"
)

// buildGuardedMux creates a mux with stub handlers for every route category,
// then wraps it with TunnelGuard using the given allowed routes.
func buildGuardedMux(allowedRoutes []string) http.Handler {
	ok := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}

	mux := http.NewServeMux()

	// home
	mux.HandleFunc("/home", ok)
	mux.HandleFunc("/login", ok)
	mux.HandleFunc("/logout", ok)
	mux.HandleFunc("/api/login", ok)
	mux.HandleFunc("/api/settings", ok)
	mux.HandleFunc("/dashboard", ok)
	mux.HandleFunc("/profile", ok)
	mux.HandleFunc("/api/dashboard/my-classes", ok)
	mux.HandleFunc("/api/v1/user/profile", ok)
	mux.HandleFunc("/api/v1/preferences", ok)

	// checkin
	mux.HandleFunc("/", ok)
	mux.HandleFunc("/kiosk", ok)
	mux.HandleFunc("/api/checkin", ok)
	mux.HandleFunc("/api/checkout", ok)
	mux.HandleFunc("/api/status", ok)
	mux.HandleFunc("/api/students/search", ok)
	mux.HandleFunc("/api/pin/check", ok)

	// tracker
	mux.HandleFunc("/api/tracker/due", ok)
	mux.HandleFunc("/api/tracker/respond", ok)

	// admin
	mux.HandleFunc("/admin", ok)
	mux.HandleFunc("/admin/login", ok)
	mux.HandleFunc("/api/admin/pin/toggle", ok)
	mux.HandleFunc("/api/attendees", ok)
	mux.HandleFunc("/api/v1/schedule/today", ok)
	mux.HandleFunc("/api/v1/directory", ok)
	mux.HandleFunc("/api/v1/tracker/items", ok)
	mux.HandleFunc("/api/v1/audit/flags", ok)
	mux.HandleFunc("/api/v1/student/profile", ok)

	// memos
	mux.HandleFunc("/memos/", ok)

	// static (uncategorized, always allowed)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return handlers.TunnelGuard(mux, allowedRoutes)
}

func tunnelReq(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Tunnel", "true")
	return req
}

func localReq(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

// TestTunnelGuard_DefaultAllowsHomeAndTracker tests that with default config
// (nil allowed_routes), home and tracker routes pass while admin, checkin,
// and memos routes are blocked.
func TestTunnelGuard_DefaultAllowsHomeAndTracker(t *testing.T) {
	handler := buildGuardedMux(nil) // defaults to ["home", "tracker"]

	allowed := []string{
		"/home", "/login", "/logout", "/api/login", "/api/settings",
		"/dashboard", "/profile", "/api/dashboard/my-classes",
		"/api/v1/user/profile", "/api/v1/preferences",
		"/api/tracker/due", "/api/tracker/respond",
	}
	for _, path := range allowed {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 200 {
			t.Errorf("tunnel request to %s: expected 200, got %d", path, w.Code)
		}
	}

	blockedAPIs := []string{
		"/api/checkin", "/api/checkout", "/api/status",
		"/api/admin/pin/toggle", "/api/attendees",
		"/api/v1/schedule/today", "/api/v1/directory",
		"/api/v1/tracker/items", "/api/v1/audit/flags",
		"/api/v1/student/profile",
	}
	for _, path := range blockedAPIs {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 403 {
			t.Errorf("tunnel request to %s: expected 403, got %d", path, w.Code)
		}
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if _, ok := resp["error"]; !ok {
			t.Errorf("tunnel request to %s: expected JSON error body", path)
		}
	}

	blockedPages := []string{"/", "/kiosk", "/admin"}
	for _, path := range blockedPages {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 302 {
			t.Errorf("tunnel request to %s: expected 302 redirect, got %d", path, w.Code)
		}
		loc := w.Header().Get("Location")
		if loc != "/home" {
			t.Errorf("tunnel request to %s: expected redirect to /home, got %s", path, loc)
		}
	}
}

// TestTunnelGuard_LocalAccessUnrestricted verifies that requests without the
// X-Tunnel header pass through to all routes regardless of config.
func TestTunnelGuard_LocalAccessUnrestricted(t *testing.T) {
	handler := buildGuardedMux(nil)

	paths := []string{
		"/home", "/", "/kiosk", "/admin",
		"/api/checkin", "/api/checkout", "/api/admin/pin/toggle",
		"/api/attendees", "/api/v1/schedule/today",
		"/api/tracker/due", "/memos/",
	}
	for _, path := range paths {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, localReq("GET", path))
		if w.Code != 200 {
			t.Errorf("local request to %s: expected 200, got %d", path, w.Code)
		}
	}
}

// TestTunnelGuard_CustomAllowedRoutes tests that configuring allowed_routes
// changes which categories are permitted through the tunnel.
func TestTunnelGuard_CustomAllowedRoutes(t *testing.T) {
	// Allow checkin and admin, block home and tracker
	handler := buildGuardedMux([]string{"checkin", "admin"})

	// Checkin routes should pass
	for _, path := range []string{"/api/checkin", "/api/checkout", "/api/status"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 200 {
			t.Errorf("tunnel request to %s: expected 200, got %d", path, w.Code)
		}
	}

	// Admin routes should pass
	for _, path := range []string{"/api/admin/pin/toggle", "/api/attendees", "/api/v1/schedule/today"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 200 {
			t.Errorf("tunnel request to %s: expected 200, got %d", path, w.Code)
		}
	}

	// Home routes should be blocked
	for _, path := range []string{"/api/login", "/api/settings"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 403 {
			t.Errorf("tunnel request to %s: expected 403, got %d", path, w.Code)
		}
	}

	// Tracker routes should be blocked
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, tunnelReq("GET", "/api/tracker/due"))
	if w.Code != 403 {
		t.Errorf("tunnel request to /api/tracker/due: expected 403, got %d", w.Code)
	}
}

// TestTunnelGuard_AllCategories tests enabling all route categories.
func TestTunnelGuard_AllCategories(t *testing.T) {
	handler := buildGuardedMux([]string{"home", "checkin", "tracker", "admin", "memos"})

	paths := []string{
		"/home", "/api/login", "/dashboard",
		"/api/checkin", "/api/checkout",
		"/api/tracker/due",
		"/api/admin/pin/toggle", "/api/v1/schedule/today",
		"/memos/",
	}
	for _, path := range paths {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 200 {
			t.Errorf("tunnel request to %s with all categories: expected 200, got %d", path, w.Code)
		}
	}
}

// TestTunnelGuard_HeaderStripped verifies that the X-Tunnel header is removed
// before reaching the application handler, preventing spoofing.
func TestTunnelGuard_HeaderStripped(t *testing.T) {
	var sawHeader bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawHeader = r.Header.Get("X-Tunnel") != ""
		w.WriteHeader(200)
	})
	guarded := handlers.TunnelGuard(inner, []string{"home"})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/home", nil)
	req.Header.Set("X-Tunnel", "true")
	guarded.ServeHTTP(w, req)

	if sawHeader {
		t.Error("X-Tunnel header should be stripped before reaching the app handler")
	}
}

// TestTunnelGuard_BlockedPageRedirect verifies that blocked page routes
// (non-API) redirect to /home instead of returning 403.
func TestTunnelGuard_BlockedPageRedirect(t *testing.T) {
	handler := buildGuardedMux(nil) // defaults: home + tracker

	pages := []string{"/", "/kiosk", "/admin", "/admin/login"}
	for _, path := range pages {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, tunnelReq("GET", path))
		if w.Code != 302 {
			t.Errorf("%s: expected 302, got %d", path, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/home" {
			t.Errorf("%s: expected redirect to /home, got %s", path, loc)
		}
	}
}

// TestTunnelGuard_BlockedAPIReturnsJSON verifies that blocked API routes
// return a 403 JSON response with an error message.
func TestTunnelGuard_BlockedAPIReturnsJSON(t *testing.T) {
	handler := buildGuardedMux(nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, tunnelReq("POST", "/api/checkin"))

	if w.Code != 403 {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	errMsg, _ := resp["error"].(string)
	if errMsg == "" {
		t.Error("expected non-empty error message in JSON response")
	}
}

// TestTunnelGuard_MemosBlocked verifies that /memos/ is blocked by default.
func TestTunnelGuard_MemosBlocked(t *testing.T) {
	handler := buildGuardedMux(nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, tunnelReq("GET", "/memos/"))
	if w.Code != 302 {
		t.Errorf("/memos/ via tunnel: expected 302, got %d", w.Code)
	}

	// But allowed when memos is in allowed_routes
	handler = buildGuardedMux([]string{"home", "tracker", "memos"})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, tunnelReq("GET", "/memos/"))
	if w.Code != 200 {
		t.Errorf("/memos/ via tunnel with memos allowed: expected 200, got %d", w.Code)
	}
}

// TestTunnelGuard_POSTMethodBlocked verifies that POST requests to blocked
// API routes are also blocked (not just GET).
func TestTunnelGuard_POSTMethodBlocked(t *testing.T) {
	handler := buildGuardedMux(nil)

	for _, path := range []string{"/api/checkin", "/api/checkout", "/api/admin/pin/toggle"} {
		w := httptest.NewRecorder()
		req := tunnelReq("POST", path)
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("POST %s via tunnel: expected 403, got %d", path, w.Code)
		}
	}
}
