package dto

import (
	"encoding/json"
	"testing"
)

// @sk-test 118-api-consistency#T4.1: ApiResponse success marshaling (AC-003)
func TestNewSuccessResponse(t *testing.T) {
	resp := NewSuccessResponse(map[string]string{"key": "value"})
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	var parsed struct {
		Data  map[string]string `json:"data"`
		Error any               `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if parsed.Data["key"] != "value" {
		t.Errorf("expected value, got %s", parsed.Data["key"])
	}
	if parsed.Error != nil {
		t.Errorf("expected nil error, got %v", parsed.Error)
	}
}

// @sk-test 118-api-consistency#T4.1: ApiResponse error marshaling (AC-004)
func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("NOT_FOUND", "resource not found")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	var parsed struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if parsed.Data != nil {
		t.Errorf("expected nil data, got %v", parsed.Data)
	}
	if parsed.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", parsed.Error.Code)
	}
	if parsed.Error.Message != "resource not found" {
		t.Errorf("expected message, got %s", parsed.Error.Message)
	}
}

// @sk-test 118-api-consistency#T4.1: ApiResponse error with validation details (AC-004)
func TestNewErrorResponseWithDetails(t *testing.T) {
	details := []ValidationDetail{
		{Field: "name", Message: "required"},
	}
	resp := NewErrorResponse("VALIDATION_ERROR", "invalid input", details...)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	var parsed struct {
		Data  any `json:"data"`
		Error struct {
			Code    string             `json:"code"`
			Message string             `json:"message"`
			Details []ValidationDetail `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if len(parsed.Error.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(parsed.Error.Details))
	}
	if parsed.Error.Details[0].Field != "name" {
		t.Errorf("expected field name, got %s", parsed.Error.Details[0].Field)
	}
}

// @sk-test 118-api-consistency#T4.1: ApiResponse paginated success (AC-005)
func TestNewSuccessPaginated(t *testing.T) {
	items := []string{"a", "b", "c"}
	resp := NewSuccessPaginated(items, 1, 20, 3)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	var parsed struct {
		Data       []string `json:"data"`
		Pagination struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"pagination"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if len(parsed.Data) != 3 {
		t.Errorf("expected 3 items, got %d", len(parsed.Data))
	}
	if parsed.Pagination.Page != 1 {
		t.Errorf("expected page 1, got %d", parsed.Pagination.Page)
	}
	if parsed.Pagination.PerPage != 20 {
		t.Errorf("expected per_page 20, got %d", parsed.Pagination.PerPage)
	}
	if parsed.Pagination.Total != 3 {
		t.Errorf("expected total 3, got %d", parsed.Pagination.Total)
	}
	if parsed.Error != nil {
		t.Errorf("expected nil error, got %v", parsed.Error)
	}
}

// @sk-test 118-api-consistency#T4.1: ApiResponse nil data when error (AC-004)
func TestApiResponseErrorHasNilData(t *testing.T) {
	resp := NewErrorResponse("INTERNAL_ERROR", "something went wrong")
	if resp.Data != nil {
		t.Error("expected Data to be nil for error responses")
	}
	if resp.Error == nil {
		t.Fatal("expected non-nil Error")
	}
	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR, got %s", resp.Error.Code)
	}
}

// @sk-test 118-api-consistency#T4.1: ApiResponse nil error when success (AC-003)
func TestApiResponseSuccessHasNilError(t *testing.T) {
	resp := NewSuccessResponse("ok")
	if resp.Error != nil {
		t.Error("expected Error to be nil for success responses")
	}
	if resp.Data != "ok" {
		t.Errorf("expected data 'ok', got %v", resp.Data)
	}
}

// @sk-test 118-api-consistency#T4.1: Pagination struct JSON tags (AC-005)
func TestPaginationJSONTags(t *testing.T) {
	p := Pagination{Page: 2, PerPage: 10, Total: 50}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	var parsed struct {
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
		Total   int `json:"total"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if parsed.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", parsed.PerPage)
	}
}
