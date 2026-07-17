package admin_session

import "errors"

// @sk-task admin-ui-design#T1.3: Admin session errors (AC-001)
var (
	ErrSessionNotFound = errors.New("admin session not found")
	ErrSessionExpired  = errors.New("admin session is expired")
)
