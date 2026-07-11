package dto

// @sk-task 60-audit-incidents#T2.1: IncidentResponse DTO (AC-002)
type IncidentResponse struct {
	ID                    string  `json:"id"`
	RequestID             string  `json:"request_id"`
	Timestamp             string  `json:"timestamp"`
	Tenant                string  `json:"tenant"`
	ProfileSlug           string  `json:"profile_slug"`
	DetectorType          string  `json:"detector_type"`
	EntryValue            *string `json:"entry_value,omitempty"`
	Severity              string  `json:"severity"`
	Action                string  `json:"action"`
	PromptSnippetRedacted *string `json:"prompt_snippet_redacted,omitempty"`
	ResponseSnippet       *string `json:"response_snippet,omitempty"`
}

// @sk-task 60-audit-incidents#T2.1: IncidentFilterParams for query binding (AC-001)
type IncidentFilterParams struct {
	Severity    string `form:"severity"`
	Tenant      string `form:"tenant"`
	ProfileSlug string `form:"profile_slug"`
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
}

// @sk-task 60-audit-incidents#T2.1: ExportQuery parameters (AC-003, AC-004, AC-008)
type ExportQuery struct {
	Format      string `form:"format"`
	Severity    string `form:"severity"`
	Tenant      string `form:"tenant"`
	ProfileSlug string `form:"profile_slug"`
}
