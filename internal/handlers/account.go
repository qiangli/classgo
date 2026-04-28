package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"classgo/internal/auth"
	memosstore "classgo/memos/store"
)

// HandleAccountList returns all identities in the current session.
func (a *App) HandleAccountList(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Not authenticated"})
		return
	}
	sess, ok := a.Sessions.Get(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Session expired"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"identities":   sess.Identities,
		"active_index": sess.ActiveIndex,
	})
}

// HandleAccountSwitch switches the active identity.
func (a *App) HandleAccountSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := auth.GetSessionToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Not authenticated"})
		return
	}

	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	if err := a.Sessions.SwitchIdentity(token, req.Index); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	sess, _ := a.Sessions.Get(token)
	active := sess.Active()
	redirect := "/home"
	if active.Role == "admin" {
		redirect = "/admin"
	} else if active.Role == "guest" {
		redirect = "/kiosk"
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": active, "redirect": redirect})
}

// HandleAccountRemove removes an identity from the session.
func (a *App) HandleAccountRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := auth.GetSessionToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Not authenticated"})
		return
	}

	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	hasMore, err := a.Sessions.RemoveIdentity(token, req.Index)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !hasMore {
		// Last identity removed — clear the session cookie.
		auth.ClearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "redirect": auth.LoginPath})
		return
	}
	sess, _ := a.Sessions.Get(token)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"identities":   sess.Identities,
		"active_index": sess.ActiveIndex,
	})
}

// HandleAccountAdd authenticates and adds a new identity to the existing session.
func (a *App) HandleAccountAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := auth.GetSessionToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Not authenticated"})
		return
	}
	if _, ok := a.Sessions.Get(token); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Session expired"})
		return
	}

	var req struct {
		Type     string `json:"type"`      // "admin", "user", or "guest"
		Username string `json:"username"`   // for admin login
		EntityID string `json:"entity_id"`  // for user login
		Password string `json:"password"`   // for admin or user login
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request"})
		return
	}

	switch req.Type {
	case "guest":
		id := auth.GuestIdentity()
		idx, err := a.Sessions.AddIdentity(token, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		a.Sessions.SwitchIdentity(token, idx)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "index": idx, "redirect": "/kiosk"})

	case "admin":
		if req.Username == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Username and password required"})
			return
		}
		if err := auth.Authenticate(req.Username, req.Password); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "Invalid credentials"})
			return
		}
		adminRole := a.getAdminRole(req.Username)
		if adminRole == "" {
			writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "Not authorized as administrator"})
			return
		}
		id := auth.Identity{
			Username:     req.Username,
			Role:         "admin",
			DisplayName:  req.Username,
			IsSuperAdmin: adminRole == "superadmin",
		}
		idx, err := a.Sessions.AddIdentity(token, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		a.Sessions.SwitchIdentity(token, idx)
		log.Printf("Account add (admin): %s (role: %s)", req.Username, adminRole)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "index": idx, "redirect": "/admin"})

	case "user":
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
		displayName := user.Nickname
		if displayName == "" {
			displayName, _ = a.lookupEntity(req.EntityID)
		}
		id := auth.Identity{
			Username:    username,
			Role:        "user",
			UserType:    userType,
			EntityID:    req.EntityID,
			DisplayName: displayName,
		}
		idx, err := a.Sessions.AddIdentity(token, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		a.Sessions.SwitchIdentity(token, idx)
		log.Printf("Account add (user): %s (%s, %s)", displayName, username, userType)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "index": idx, "redirect": "/home"})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid type, must be 'admin', 'user', or 'guest'"})
	}
}
