package auth

import (
	"crypto/rand"
	"encoding/hex"
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

type Session struct {
	Username  string
	Role      string // "admin" or "user"
	UserType  string // "student", "parent", "teacher", "" (admin)
	EntityID  string // original entity ID (e.g., "S001")
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]Session)}
}

func (s *SessionStore) Create(username, role, userType, entityID string) string {
	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = Session{
		Username:  username,
		Role:      role,
		UserType:  userType,
		EntityID:  entityID,
		ExpiresAt: time.Now().Add(SessionMaxAge),
	}
	s.mu.Unlock()
	return token
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
	return sess, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
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
