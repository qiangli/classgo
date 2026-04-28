package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	SessionCookie  = "classgo_session"
	SessionMaxAge  = 8 * time.Hour
	LoginPath      = "/login"
	AdminLoginPath = "/admin/login"
)

// Identity represents a single authenticated user within a session.
type Identity struct {
	Username     string `json:"username"`
	Role         string `json:"role"`      // "admin", "user", or "guest"
	UserType     string `json:"user_type"` // "student", "parent", "teacher", ""
	EntityID     string `json:"entity_id"`
	IsSuperAdmin bool   `json:"is_super_admin,omitempty"`
	DisplayName  string `json:"display_name"`
}

// Session holds one or more authenticated identities. The top-level fields
// (Username, Role, etc.) always reflect the active identity for backward
// compatibility — existing code can read sess.Role without changes.
type Session struct {
	// Active identity fields (synced from Identities[ActiveIndex]).
	Username     string
	Role         string // "admin", "user", or "guest"
	UserType     string // "student", "parent", "teacher", "" (admin/guest)
	EntityID     string // original entity ID (e.g., "S001")
	IsSuperAdmin bool   // true if this admin has superadmin privileges

	// Multi-identity support.
	Identities  []Identity
	ActiveIndex int
	ExpiresAt   time.Time
}

// syncActive copies the active identity's fields to the top-level Session fields.
func (s *Session) syncActive() {
	if len(s.Identities) == 0 {
		return
	}
	if s.ActiveIndex < 0 || s.ActiveIndex >= len(s.Identities) {
		s.ActiveIndex = 0
	}
	id := s.Identities[s.ActiveIndex]
	s.Username = id.Username
	s.Role = id.Role
	s.UserType = id.UserType
	s.EntityID = id.EntityID
	s.IsSuperAdmin = id.IsSuperAdmin
}

// Active returns the currently active identity.
func (s *Session) Active() Identity {
	if len(s.Identities) == 0 {
		return Identity{}
	}
	if s.ActiveIndex < 0 || s.ActiveIndex >= len(s.Identities) {
		return s.Identities[0]
	}
	return s.Identities[s.ActiveIndex]
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]Session)}
}

// Create creates a new session with a single identity.
func (s *SessionStore) Create(username, role, userType, entityID string) string {
	return s.CreateWithDisplayName(username, role, userType, entityID, "")
}

// CreateWithDisplayName creates a new session with a single identity including a display name.
func (s *SessionStore) CreateWithDisplayName(username, role, userType, entityID, displayName string) string {
	token := generateToken()
	id := Identity{
		Username:    username,
		Role:        role,
		UserType:    userType,
		EntityID:    entityID,
		DisplayName: displayName,
	}
	sess := Session{
		Identities:  []Identity{id},
		ActiveIndex: 0,
		ExpiresAt:   time.Now().Add(SessionMaxAge),
	}
	sess.syncActive()
	s.mu.Lock()
	s.sessions[token] = sess
	s.mu.Unlock()
	return token
}

// SetSuperAdmin marks the active identity as superadmin.
func (s *SessionStore) SetSuperAdmin(token string) {
	s.mu.Lock()
	if sess, ok := s.sessions[token]; ok {
		if len(sess.Identities) > 0 && sess.ActiveIndex < len(sess.Identities) {
			sess.Identities[sess.ActiveIndex].IsSuperAdmin = true
		}
		sess.IsSuperAdmin = true
		s.sessions[token] = sess
	}
	s.mu.Unlock()
}

func (s *SessionStore) Get(token string) (Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok || time.Now().After(sess.ExpiresAt) {
		if ok {
			s.Delete(token)
		}
		return Session{}, false
	}
	sess.syncActive()
	return sess, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// AddIdentity adds a new identity to an existing session. If an identity with
// the same username and role already exists, it is replaced. Returns the index
// of the added/updated identity.
func (s *SessionStore) AddIdentity(token string, id Identity) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return -1, fmt.Errorf("session not found")
	}
	// Replace if same username+role already exists.
	for i, existing := range sess.Identities {
		if existing.Username == id.Username && existing.Role == id.Role {
			sess.Identities[i] = id
			s.sessions[token] = sess
			return i, nil
		}
	}
	sess.Identities = append(sess.Identities, id)
	s.sessions[token] = sess
	return len(sess.Identities) - 1, nil
}

// SwitchIdentity changes the active identity. Returns an error if the index is out of range.
func (s *SessionStore) SwitchIdentity(token string, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return fmt.Errorf("session not found")
	}
	if index < 0 || index >= len(sess.Identities) {
		return fmt.Errorf("identity index %d out of range (have %d)", index, len(sess.Identities))
	}
	sess.ActiveIndex = index
	sess.syncActive()
	s.sessions[token] = sess
	return nil
}

// RemoveIdentity removes an identity by index. Returns true if the session
// still has identities, false if it was the last one (session is deleted).
func (s *SessionStore) RemoveIdentity(token string, index int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return false, fmt.Errorf("session not found")
	}
	if index < 0 || index >= len(sess.Identities) {
		return false, fmt.Errorf("identity index %d out of range (have %d)", index, len(sess.Identities))
	}
	sess.Identities = append(sess.Identities[:index], sess.Identities[index+1:]...)
	if len(sess.Identities) == 0 {
		delete(s.sessions, token)
		return false, nil
	}
	// Adjust active index.
	if sess.ActiveIndex >= len(sess.Identities) {
		sess.ActiveIndex = len(sess.Identities) - 1
	} else if sess.ActiveIndex > index {
		sess.ActiveIndex--
	}
	sess.syncActive()
	s.sessions[token] = sess
	return true, nil
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SetSessionCookie sets the session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// GetSessionToken extracts the session token from the request cookie.
func GetSessionToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// GuestIdentity returns a pre-built guest (kiosk) identity.
func GuestIdentity() Identity {
	return Identity{
		Username:    "guest",
		Role:        "guest",
		UserType:    "",
		EntityID:    "",
		DisplayName: "Guest (Kiosk)",
	}
}
