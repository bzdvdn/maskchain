package session

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task sessions#T1.1: Create Session entity with all fields (AC-001, AC-009)
type Session struct {
	SessionID         string
	TenantID          string
	Model             string
	TokenCount        int64
	MessageCount      int32
	TotalMasks        int32
	DictMaskCount     int32
	PIIMaskCount      int32
	PreprocessorCount int32
	Status            SessionStatus
	TTL               time.Duration
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusExpired SessionStatus = "expired"
	SessionStatusClosed  SessionStatus = "closed"
)

func NewSessionID() string {
	return mask.NewUUIDv7()
}
