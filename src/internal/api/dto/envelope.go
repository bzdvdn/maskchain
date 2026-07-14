package dto

// @sk-task 118-api-consistency#T1.1: ApiResponse unified envelope type (AC-003, AC-004, AC-005)
type ApiResponse struct {
	Data       any         `json:"data"`
	Error      *ErrorInfo  `json:"error"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// @sk-task 118-api-consistency#T1.1: ErrorInfo structured error in envelope (AC-004)
type ErrorInfo struct {
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Details []ValidationDetail `json:"details,omitempty"`
}

// @sk-task 118-api-consistency#T1.1: Pagination metadata (AC-005)
type Pagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

func NewSuccessResponse(data any) ApiResponse {
	return ApiResponse{Data: data, Error: nil}
}

func NewSuccessPaginated(data any, page, perPage, total int) ApiResponse {
	return ApiResponse{
		Data:  data,
		Error: nil,
		Pagination: &Pagination{
			Page:    page,
			PerPage: perPage,
			Total:   total,
		},
	}
}

func NewErrorResponse(code, message string, details ...ValidationDetail) ApiResponse {
	ei := &ErrorInfo{Code: code, Message: message}
	if len(details) > 0 {
		ei.Details = details
	}
	return ApiResponse{Data: nil, Error: ei}
}
