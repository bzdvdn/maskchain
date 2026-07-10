package mask

import "errors"

// @sk-task 22-shield-mask-storage#T1.1: Implement sentinel errors (AC-006)
var (
	ErrMaskNotFound   = errors.New("mask entry not found")
	ErrMaskIDConflict = errors.New("mask ID already exists")
)
