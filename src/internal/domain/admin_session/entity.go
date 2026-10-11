package admin_session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// @sk-task admin-ui-design#T1.3: AdminSession entity (AC-001)
//
// AdminSession represents a domain entity or configuration.
type AdminSession struct {
	ID        string
	Username  string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// GenerateToken produces a cryptographically random hex token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of the token.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// NewAdminSession creates a session and returns the raw token (the session secret).
// The caller must give the raw token to the client; only the hash is persisted.
func NewAdminSession(username string, ttl time.Duration) (*AdminSession, string, error) {
	token, err := GenerateToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	return &AdminSession{
		ID:        NewAdminSessionID(),
		Username:  username,
		TokenHash: HashToken(token),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}, token, nil
}

// NewAdminSessionID returns a short ID used as the DB primary key (not a secret).
func NewAdminSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// @sk-task admin-session-sliding#T1.1: Sliding renewal once past half-lifetime (AC-001, AC-003)
//
// Renew extends the session back to its original lifetime when less than half
// of that lifetime remains. It returns false for expired sessions and for
// sessions that are still fresh (more than half remaining), so the store is not
// written on every request.
func (s *AdminSession) Renew(now time.Time) bool {
	lifetime := s.ExpiresAt.Sub(s.CreatedAt)
	if lifetime <= 0 {
		return false
	}
	if !now.Before(s.ExpiresAt) {
		return false
	}
	if s.ExpiresAt.Sub(now) >= lifetime/2 {
		return false
	}
	s.ExpiresAt = now.Add(lifetime)
	return true
}
