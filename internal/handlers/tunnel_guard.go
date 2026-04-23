package handlers

import (
	"net/http"
	"strings"
)

// tunnelHeader is the custom header injected by frpc to identify tunnel traffic.
const tunnelHeader = "X-Tunnel"

// defaultAllowedRoutes are the route categories allowed via tunnel when none are configured.
var defaultAllowedRoutes = []string{"home", "tracker"}

// routeCategories maps each category name to a route matcher function.
var routeCategories = map[string]func(path string) bool{
	"home": func(path string) bool {
		switch path {
		case "/home", "/login", "/logout", "/api/login", "/api/users/search", "/api/settings",
			"/dashboard", "/profile", "/api/v1/user/profile", "/api/v1/preferences":
			return true
		}
		return strings.HasPrefix(path, "/api/dashboard/") ||
			strings.HasPrefix(path, "/static/")
	},
	"checkin": func(path string) bool {
		switch path {
		case "/", "/kiosk", "/api/checkin", "/api/checkout", "/api/status",
			"/api/students/search", "/api/pin/check":
			return true
		}
		return strings.HasPrefix(path, "/api/student/pin/")
	},
	"tracker": func(path string) bool {
		return strings.HasPrefix(path, "/api/tracker/")
	},
	"admin": func(path string) bool {
		return strings.HasPrefix(path, "/admin") ||
			strings.HasPrefix(path, "/api/admin/") ||
			strings.HasPrefix(path, "/api/attendees") ||
			strings.HasPrefix(path, "/api/v1/schedule/") ||
			strings.HasPrefix(path, "/api/v1/directory") ||
			strings.HasPrefix(path, "/api/v1/import") ||
			strings.HasPrefix(path, "/api/v1/data") ||
			strings.HasPrefix(path, "/api/v1/password-reset") ||
			strings.HasPrefix(path, "/api/v1/memos/") ||
			strings.HasPrefix(path, "/api/v1/tracker/") ||
			strings.HasPrefix(path, "/api/v1/admin/") ||
			strings.HasPrefix(path, "/api/v1/student/pin/") ||
			strings.HasPrefix(path, "/api/v1/student/profile") ||
			strings.HasPrefix(path, "/api/v1/audit/")
	},
	"memos": func(path string) bool {
		return strings.HasPrefix(path, "/memos/") || strings.HasPrefix(path, "/memos")
	},
}

// TunnelGuard returns middleware that restricts routes for requests arriving
// through the frp tunnel (identified by the X-Tunnel header). Only routes
// matching the allowed categories are passed through; others get 403.
func TunnelGuard(next http.Handler, allowedRoutes []string) http.Handler {
	if len(allowedRoutes) == 0 {
		allowedRoutes = defaultAllowedRoutes
	}

	allowed := make(map[string]bool, len(allowedRoutes))
	for _, r := range allowedRoutes {
		allowed[r] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(tunnelHeader) != "true" {
			next.ServeHTTP(w, r)
			return
		}

		// Strip the X-Tunnel header so application code can't be spoofed
		// by a local client setting it manually. Only frpc should set this.
		r.Header.Del(tunnelHeader)

		path := r.URL.Path

		for category, matcher := range routeCategories {
			if matcher(path) {
				if allowed[category] {
					next.ServeHTTP(w, r)
					return
				}
				// Blocked route
				if isAPIRoute(path) {
					writeJSON(w, http.StatusForbidden, map[string]any{
						"error": "This endpoint is not available via public access",
					})
				} else {
					http.Redirect(w, r, "/home", http.StatusFound)
				}
				return
			}
		}

		// Routes not in any category (e.g. /static/) — allow by default
		next.ServeHTTP(w, r)
	})
}

func isAPIRoute(path string) bool {
	return strings.HasPrefix(path, "/api/")
}
