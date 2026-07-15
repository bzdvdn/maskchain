package session

import "errors"

// @sk-task sessions#T1.1: Implement sentinel errors (AC-001)
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session is expired")
	ErrSessionClosed   = errors.New("session is closed")
	ErrSessionConflict = errors.New("session ID already exists")
)
