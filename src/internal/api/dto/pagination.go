package dto

// @sk-task 41-profiles-ui#T2.1: PaginatedResponse for list endpoints (AC-002)
// @sk-task 118-api-consistency#T1.1: Updated to use PerPage and Pagination struct (AC-005)
//
// PaginatedResponse represents a domain entity or configuration.
type PaginatedResponse struct {
	Data     any `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	PerPage  int `json:"per_page,omitempty"`
}
