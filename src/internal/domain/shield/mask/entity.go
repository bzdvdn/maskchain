package mask

import (
	"fmt"
	"strings"
	"time"
)

// @sk-task mask-token-format#T1.2: Format enum selects token style (AC-001, AC-002, AC-007)
//
// Format controls how masked tokens are rendered.
type Format int

const (
	// FormatClean emits counter-only tokens like [MASK.1] — no document id.
	FormatClean Format = iota
	// FormatID emits the legacy [MASK_<docID>.<N>] tokens.
	FormatID
	// FormatRedact emits a fixed [REDACTED] token and marks the entry non-reversible.
	FormatRedact
)

// @sk-task mask-token-format#T1.2: ParseFormat maps a query value to a Format (AC-003)
//
// ParseFormat resolves a /mask format query value; empty means FormatClean.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "clean":
		return FormatClean, nil
	case "id":
		return FormatID, nil
	case "redact":
		return FormatRedact, nil
	default:
		return FormatClean, fmt.Errorf("invalid format %q: allowed values are clean, id, redact", s)
	}
}

// @sk-task 22-shield-mask-storage#T1.1: Create MaskEntry entity (AC-001, AC-002)
// @sk-task cleanup-profile-repository#T3.4: Remove ProfileID field (AC-010)
// @sk-task mask-token-format#T1.2: Reversible flag on MaskEntry (AC-007)
//
// MaskEntry represents a domain entity or configuration.
type MaskEntry struct {
	MaskID         string
	DocumentMaskID string
	Replacements   map[string]string
	Reversible     bool
	CreatedAt      time.Time
}
