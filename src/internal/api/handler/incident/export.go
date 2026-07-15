package incident

import (
	"encoding/csv"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task cleanup-profile-repository#T3.2: Remove ProfileSlug from export (AC-012, AC-013)
// @sk-task 60-audit-incidents#T2.2: Export incidents as CSV or JSON (AC-003, AC-004, AC-008)
func (h *Handler) ExportIncidents(c *gin.Context) {
	var params struct {
		Format   string `form:"format"`
		Severity string `form:"severity"`
		Tenant   string `form:"tenant"`
	}
	if err := c.ShouldBindQuery(&params); err != nil {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "invalid query parameters")
		return
	}

	format := params.Format
	if format != "csv" && format != "json" {
		middleware.AbortWithError(c, http.StatusBadRequest, middleware.ErrorCodeValidationError, "format must be 'csv' or 'json'")
		return
	}

	filter := shield.IncidentFilter{
		Page:     1,
		PageSize: 0,
	}
	if params.Severity != "" {
		s := params.Severity
		filter.Severity = &s
	}
	if params.Tenant != "" {
		t := params.Tenant
		filter.Tenant = &t
	}

	incidents, _, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		middleware.AbortWithError(c, http.StatusInternalServerError, middleware.ErrorCodeInternal, "failed to export incidents")
		return
	}

	if format == "csv" {
		exportCSV(c, incidents)
	} else {
		exportJSON(c, incidents)
	}
}

func exportCSV(c *gin.Context, incidents []*entity.Incident) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=incidents.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	header := []string{"id", "request_id", "timestamp", "tenant", "detector_type", "entry_value", "severity", "action", "prompt_snippet_redacted", "response_snippet"}
	if err := writer.Write(header); err != nil {
		return
	}

	for _, inc := range incidents {
		row := []string{
			inc.Slug(),
			inc.RequestID(),
			inc.Timestamp().Format(time.RFC3339),
			inc.Tenant(),
			inc.DetectorType(),
			valueOrEmpty(inc.EntryValue()),
			inc.Severity().String(),
			inc.Action(),
			valueOrEmpty(inc.PromptSnippetRedacted()),
			valueOrEmpty(inc.ResponseSnippet()),
		}
		if err := writer.Write(row); err != nil {
			return
		}
	}
}

func exportJSON(c *gin.Context, incidents []*entity.Incident) {
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=incidents.json")

	resp := make([]dtoIncidentExport, 0, len(incidents))
	for _, inc := range incidents {
		resp = append(resp, dtoIncidentExport{
			ID:                    inc.Slug(),
			RequestID:             inc.RequestID(),
			Timestamp:             inc.Timestamp().Format(time.RFC3339),
			Tenant:                inc.Tenant(),
			DetectorType:          inc.DetectorType(),
			EntryValue:            inc.EntryValue(),
			Severity:              inc.Severity().String(),
			Action:                inc.Action(),
			PromptSnippetRedacted: inc.PromptSnippetRedacted(),
			ResponseSnippet:       inc.ResponseSnippet(),
		})
	}

	c.JSON(http.StatusOK, resp)
}

type dtoIncidentExport struct {
	ID                    string  `json:"id"`
	RequestID             string  `json:"request_id"`
	Timestamp             string  `json:"timestamp"`
	Tenant                string  `json:"tenant"`
	DetectorType          string  `json:"detector_type"`
	EntryValue            *string `json:"entry_value,omitempty"`
	Severity              string  `json:"severity"`
	Action                string  `json:"action"`
	PromptSnippetRedacted *string `json:"prompt_snippet_redacted,omitempty"`
	ResponseSnippet       *string `json:"response_snippet,omitempty"`
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
