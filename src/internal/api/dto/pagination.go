package dto

// @sk-task 41-profiles-ui#T2.1: PaginatedResponse for list endpoints (AC-002)
type PaginatedResponse struct {
	Data     any   `json:"data"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
