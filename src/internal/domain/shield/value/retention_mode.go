package value

import "fmt"

// @sk-task 402-zero-retention-mode#T1.2: Implement RetentionMode value object (DM-001)
//
// RetentionMode is a string type for domain values.
type RetentionMode string

const (
	RetentionModeFull RetentionMode = "full"
	RetentionModeMeta RetentionMode = "meta"
	RetentionModeNone RetentionMode = "none"
)

// ParseRetentionMode validates v against the known enum.
func ParseRetentionMode(v string) (RetentionMode, error) {
	switch RetentionMode(v) {
	case RetentionModeFull, RetentionModeMeta, RetentionModeNone:
		return RetentionMode(v), nil
	default:
		return "", fmt.Errorf("invalid retention mode %q (want full|meta|none)", v)
	}
}

func (m RetentionMode) String() string { return string(m) }

func (m RetentionMode) Valid() bool {
	return m == RetentionModeFull || m == RetentionModeMeta || m == RetentionModeNone
}
