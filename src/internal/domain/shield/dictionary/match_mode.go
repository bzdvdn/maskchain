package dictionary

// @sk-task 24-shield-dictionaries#T1.1: Implement MatchMode type (AC-001)
type MatchMode string

const (
	MatchModeExact    MatchMode = "exact"
	MatchModeContains MatchMode = "contains"
	MatchModeRegex    MatchMode = "regex"
	MatchModeFuzzy    MatchMode = "fuzzy"
)

func (m MatchMode) String() string { return string(m) }
