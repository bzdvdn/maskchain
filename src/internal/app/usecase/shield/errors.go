package shield

import "errors"

// @sk-task 50-shield-engine#T1.1: Define ErrProfileNotFound and ErrProfileDisabled sentinels
var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrProfileDisabled = errors.New("profile is disabled")
)
