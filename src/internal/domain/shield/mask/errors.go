package mask

import "errors"

// @sk-task 22-shield-mask-storage#T1.1: Implement sentinel errors (AC-006)
// @sk-task mask-token-format#T1.2: ErrNotReversible for redact entries (AC-007)
//
// Sentinel errors returned to callers of the mask domain.
var (
	ErrMaskNotFound   = errors.New("mask entry not found")
	ErrMaskIDConflict = errors.New("mask ID already exists")
	ErrNotReversible  = errors.New("mask entry is not reversible")
)
