package mask

import "time"

// @sk-task 22-shield-mask-storage#T1.1: Create MaskEntry entity (AC-001, AC-002)
type MaskEntry struct {
	MaskID       string
	ProfileID    *string
	Replacements map[string]string
	CreatedAt    time.Time
}
