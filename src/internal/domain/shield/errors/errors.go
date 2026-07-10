package errors

import "errors"

// @sk-task 20-shield-domain#T1.2: Implement sentinel domain errors (AC-007)
var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrInvalidPattern  = errors.New("invalid pattern")
	ErrInvalidSlug     = errors.New("invalid profile slug")
	ErrDetectorFailed  = errors.New("detector execution failed")
	ErrDuplicateSlug   = errors.New("duplicate profile slug")
)
