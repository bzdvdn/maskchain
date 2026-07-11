package incident

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

type mockIncidentRepo struct {
	incidents []*entity.Incident
}

func (r *mockIncidentRepo) Save(_ context.Context, _ *entity.Incident) error {
	return nil
}

func (r *mockIncidentRepo) FindByID(_ context.Context, id string) (*entity.Incident, error) {
	for _, inc := range r.incidents {
		if inc.Slug() == id {
			return inc, nil
		}
	}
	return nil, nil
}

func (r *mockIncidentRepo) ListByProfile(_ context.Context, _ value.ProfileID) ([]*entity.Incident, error) {
	return nil, nil
}

func (r *mockIncidentRepo) ListByTenant(_ context.Context, _ value.TenantID) ([]*entity.Incident, error) {
	return nil, nil
}

func (r *mockIncidentRepo) List(_ context.Context, filter shield.IncidentFilter) ([]*entity.Incident, int, error) {
	var result []*entity.Incident
	for _, inc := range r.incidents {
		if filter.Severity != nil && inc.Severity().String() != *filter.Severity {
			continue
		}
		if filter.Tenant != nil && inc.Tenant() != *filter.Tenant {
			continue
		}
		if filter.ProfileSlug != nil && inc.ProfileSlug() != *filter.ProfileSlug {
			continue
		}
		result = append(result, inc)
	}

	total := len(result)
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return result[start:end], total, nil
}

func setupTestIncidents() []*entity.Incident {
	now := time.Now()
	sevCritical := value.SeverityCritical
	sevHigh := value.SeverityHigh
	sevMedium := value.SeverityMedium

	entry := "test-value"
	prompt := "redacted-prompt"
	resp := "response-data"

	return []*entity.Incident{
		entity.NewAuditIncident("inc-001", "prof-a", "req-1", "regex", &entry, sevCritical, "block", &prompt, &resp, "tenant-1", now),
		entity.NewAuditIncident("inc-002", "prof-b", "req-2", "dictionary", nil, sevHigh, "redact", &prompt, nil, "tenant-1", now.Add(-time.Hour)),
		entity.NewAuditIncident("inc-003", "prof-a", "req-3", "regex", nil, sevMedium, "alert", nil, nil, "tenant-2", now.Add(-2*time.Hour)),
	}
}

// @sk-test 60-audit-incidents#T4.2: TestListIncidents returns paginated results (AC-001, AC-006)
func TestListIncidents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockIncidentRepo{incidents: setupTestIncidents()}
	h := New(repo)

	t.Run("empty database", func(t *testing.T) {
		emptyRepo := &mockIncidentRepo{}
		emptyH := New(emptyRepo)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents", nil)

		emptyH.ListIncidents(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp dto.PaginatedResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Total != 0 {
			t.Errorf("expected total 0, got %d", resp.Total)
		}
		if len(resp.Data.([]interface{})) != 0 {
			t.Errorf("expected empty data, got %d items", len(resp.Data.([]interface{})))
		}
	})

	t.Run("filter by severity", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents?severity=critical&page=1&page_size=2", nil)

		h.ListIncidents(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp dto.PaginatedResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Total != 1 {
			t.Errorf("expected total 1, got %d", resp.Total)
		}
		if resp.Page != 1 || resp.PageSize != 2 {
			t.Errorf("unexpected pagination: page=%d, page_size=%d", resp.Page, resp.PageSize)
		}
	})
}

// @sk-test 60-audit-incidents#T4.2: TestGetIncident returns detail or 404 (AC-002, AC-007)
func TestGetIncident(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockIncidentRepo{incidents: setupTestIncidents()}
	h := New(repo)

	t.Run("existing incident", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents/inc-001", nil)
		c.Params = []gin.Param{{Key: "id", Value: "inc-001"}}

		h.GetIncident(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp dto.IncidentResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.ID != "inc-001" {
			t.Errorf("expected inc-001, got %s", resp.ID)
		}
		if resp.Severity != "critical" {
			t.Errorf("expected critical, got %s", resp.Severity)
		}
		if resp.Tenant != "tenant-1" {
			t.Errorf("expected tenant-1, got %s", resp.Tenant)
		}
		if resp.PromptSnippetRedacted == nil || *resp.PromptSnippetRedacted != "redacted-prompt" {
			t.Errorf("expected prompt snippet, got %v", resp.PromptSnippetRedacted)
		}
		if resp.ResponseSnippet == nil || *resp.ResponseSnippet != "response-data" {
			t.Errorf("expected response snippet, got %v", resp.ResponseSnippet)
		}
	})

	t.Run("nonexistent incident", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents/nonexistent", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		h.GetIncident(c)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		var errResp dto.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if errResp.Code != "NOT_FOUND" {
			t.Errorf("expected NOT_FOUND code, got %s", errResp.Code)
		}
	})
}

// @sk-test 60-audit-incidents#T4.2: TestExportIncidents exports CSV and JSON (AC-003, AC-004, AC-008)
func TestExportIncidents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockIncidentRepo{incidents: setupTestIncidents()}
	h := New(repo)

	t.Run("export csv", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents/export?format=csv", nil)

		h.ExportIncidents(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if ct != "text/csv" {
			t.Errorf("expected text/csv, got %s", ct)
		}
		body := w.Body.String()
		if !strings.HasPrefix(body, "id,request_id,timestamp") {
			t.Errorf("expected CSV header, got: %s", body[:40])
		}
	})

	t.Run("export json", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents/export?format=json", nil)

		h.ExportIncidents(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		var arr []interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(arr) != 3 {
			t.Errorf("expected 3 incidents, got %d", len(arr))
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/incidents/export?format=xml", nil)

		h.ExportIncidents(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestExportIncidents_Filtered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mockIncidentRepo{incidents: setupTestIncidents()}
	h := New(repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/incidents/export?format=json&severity=critical", nil)

	h.ExportIncidents(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var arr []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 critical incident, got %d", len(arr))
	}
}
