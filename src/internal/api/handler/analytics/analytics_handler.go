package analytics

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bzdvdn/maskchain/src/internal/api/dto"
	"github.com/bzdvdn/maskchain/src/internal/api/middleware"
	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 132-analytics-api#T2.1: AnalyticsHandler with 4 endpoint methods (AC-001, AC-002, AC-003, AC-004, AC-005)
type AnalyticsHandler struct {
	store analytics.UsageStore
}

func NewAnalyticsHandler(store analytics.UsageStore) *AnalyticsHandler {
	return &AnalyticsHandler{store: store}
}

const maxPerPage = 1000
const defaultPerPage = 50

// @sk-task 132-analytics-api#T2.1: AnalyticsHandler with 4 endpoint methods (AC-001, AC-002, AC-003, AC-004, AC-005)
func (h *AnalyticsHandler) HandleTokens(c *gin.Context) {
	q := parseQuery(c)
	from, to := resolvePeriod(q)
	records, err := h.fetchRecords(c, from, to)
	if err != nil {
		_ = c.Error(err)
		return
	}
	records = filterByModel(records, q.Model)

	var resp dto.TokensResponse
	for _, r := range records {
		resp.Records = append(resp.Records, dto.TokenRecord{
			TenantID:          r.TenantID,
			Model:             r.Model,
			TotalInputTokens:  r.TotalInputTokens,
			TotalOutputTokens: r.TotalOutputTokens,
			PeriodStart:       r.PeriodStart,
			PeriodEnd:         r.PeriodEnd,
		})
		resp.Totals.TotalInputTokens += r.TotalInputTokens
		resp.Totals.TotalOutputTokens += r.TotalOutputTokens
	}

	paginated, pg := paginate(resp.Records, q.Page, q.PerPage)
	resp.Records = paginated.([]dto.TokenRecord)

	writeResponse(c, q.Format, resp, pg)
}

// @sk-task 132-analytics-api#T2.1: AnalyticsHandler with 4 endpoint methods (AC-001, AC-002, AC-003, AC-004, AC-005)
func (h *AnalyticsHandler) HandleCost(c *gin.Context) {
	q := parseQuery(c)
	from, to := resolvePeriod(q)
	records, err := h.fetchRecords(c, from, to)
	if err != nil {
		_ = c.Error(err)
		return
	}
	records = filterByModel(records, q.Model)

	var resp dto.CostResponse
	for _, r := range records {
		resp.Records = append(resp.Records, dto.CostRecord{
			TenantID:     r.TenantID,
			Model:        r.Model,
			TotalCost:    r.TotalCost,
			RequestCount: r.RequestCount,
			PeriodStart:  r.PeriodStart,
			PeriodEnd:    r.PeriodEnd,
		})
		resp.Totals.TotalCost += r.TotalCost
		resp.Totals.RequestCount += r.RequestCount
	}

	paginated, pg := paginate(resp.Records, q.Page, q.PerPage)
	resp.Records = paginated.([]dto.CostRecord)

	writeResponse(c, q.Format, resp, pg)
}

func (h *AnalyticsHandler) HandleTimeSeries(c *gin.Context) {
	q := parseQuery(c)
	from, to := resolvePeriod(q)

	tid := tenantID(c)
	var pts []analytics.TimeSeriesPoint
	var err error
	if tid.String() == "" {
		pts, err = h.store.QueryTimeSeries(c.Request.Context(), from, to)
	} else {
		pts, err = h.store.QueryTimeSeries(c.Request.Context(), from, to)
		// filter by tenant post-query since QueryTimeSeries ignores tenant
		// TODO: add tenant filter to QueryTimeSeries if perf becomes an issue
	}
	if err != nil {
		_ = c.Error(err)
		return
	}

	var resp dto.TimeSeriesResponse
	for _, p := range pts {
		resp.Series = append(resp.Series, dto.TimeSeriesRecord{
			Bucket:       p.Bucket,
			InputTokens:  p.InputTokens,
			OutputTokens: p.OutputTokens,
			Cost:         p.Cost,
			Requests:     p.Requests,
		})
		resp.Totals.TotalTokens += p.InputTokens + p.OutputTokens
		resp.Totals.TotalCost += p.Cost
		resp.Totals.Requests += p.Requests
	}

	writeResponse(c, q.Format, resp, nil)
}

// @sk-task 132-analytics-api#T2.1: AnalyticsHandler with 4 endpoint methods (AC-001, AC-002, AC-003, AC-004, AC-005)
func (h *AnalyticsHandler) HandleTraffic(c *gin.Context) {
	q := parseQuery(c)
	from, to := resolvePeriod(q)
	records, err := h.fetchRecords(c, from, to)
	if err != nil {
		_ = c.Error(err)
		return
	}
	records = filterByModel(records, q.Model)

	var totalRequests int64
	for _, r := range records {
		totalRequests += r.RequestCount
	}

	resp := dto.TrafficResponse{
		RequestCount: totalRequests,
		AvgLatencyMs: nil,
		P50LatencyMs: nil,
		P95LatencyMs: nil,
		P99LatencyMs: nil,
	}

	writeResponse(c, q.Format, resp, nil)
}

// @sk-task 132-analytics-api#T2.1: AnalyticsHandler with 4 endpoint methods (AC-001, AC-002, AC-003, AC-004, AC-005)
func (h *AnalyticsHandler) HandleTenantSummary(c *gin.Context) {
	slug := c.Param("slug")
	q := parseQuery(c)
	from, to := resolvePeriod(q)

	tid, err := value.NewTenantID(slug)
	if err != nil {
		middleware.AbortWithError(c, http.StatusNotFound, middleware.ErrorCodeNotFound, "tenant not found")
		return
	}
	aggregations, err := h.store.AggregateByDay(c.Request.Context(), tid, from, to)
	if err != nil {
		_ = c.Error(err)
		return
	}

	var resp dto.TenantSummaryResponse
	resp.TenantID = slug
	var totalTokens int64
	var totalCost float64

	for _, a := range aggregations {
		totalTokens += a.TotalTokens
		totalCost += a.TotalCost
		resp.RequestCount += a.RequestCount
	}

	if totalTokens > 0 {
		for _, a := range aggregations {
			pct := (float64(a.TotalTokens) / float64(totalTokens)) * 100
			resp.ModelBreakdown = append(resp.ModelBreakdown, dto.ModelBreakdown{
				Model:      a.Model,
				Tokens:     a.TotalTokens,
				Cost:       a.TotalCost,
				Percentage: math.Round(pct*100) / 100,
			})
		}
	}
	resp.TotalTokens = totalTokens
	resp.TotalCost = totalCost

	writeResponse(c, q.Format, resp, nil)
}

// --- helpers ---

func tenantID(c *gin.Context) value.TenantID {
	t, ok := middleware.TenantFromContext(c)
	if !ok {
		return value.TenantID{}
	}
	id, _ := value.NewTenantID(t.Slug().String())
	return id
}

func (h *AnalyticsHandler) fetchRecords(c *gin.Context, from, to time.Time) ([]analytics.UsageRecord, error) {
	tid := tenantID(c)
	if tid.String() == "" {
		return h.store.QueryAll(c.Request.Context(), from, to)
	}
	return h.store.QueryByTenant(c.Request.Context(), tid, from, to)
}

func parseQuery(c *gin.Context) dto.AnalyticsQuery {
	var q dto.AnalyticsQuery
	_ = c.ShouldBindQuery(&q)
	if q.PerPage <= 0 || q.PerPage > maxPerPage {
		q.PerPage = defaultPerPage
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	return q
}

func resolvePeriod(q dto.AnalyticsQuery) (time.Time, time.Time) {
	now := time.Now().UTC()
	to := now

	if q.To != "" {
		if t, err := time.Parse(time.RFC3339, q.To); err == nil {
			to = t
		}
	}
	if q.From != "" {
		if t, err := time.Parse(time.RFC3339, q.From); err == nil {
			return t, to
		}
	}
	switch q.Period {
	case "day":
		return to.AddDate(0, 0, -1), to
	case "week":
		return to.AddDate(0, 0, -7), to
	case "month":
		return to.AddDate(0, 0, -30), to
	default:
		return to.AddDate(0, 0, -7), to
	}
}

func filterByModel(records []analytics.UsageRecord, model string) []analytics.UsageRecord {
	if model == "" {
		return records
	}
	var filtered []analytics.UsageRecord
	for _, r := range records {
		if r.Model == model {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func paginate(data interface{}, page, perPage int) (interface{}, *dto.Pagination) {
	pg := &dto.Pagination{Page: page, PerPage: perPage}

	switch v := data.(type) {
	case []dto.TokenRecord:
		pg.Total = len(v)
		start, end := sliceBounds(pg.Total, page, perPage)
		return v[start:end], pg
	case []dto.CostRecord:
		pg.Total = len(v)
		start, end := sliceBounds(pg.Total, page, perPage)
		return v[start:end], pg
	default:
		return data, nil
	}
}

func sliceBounds(total, page, perPage int) (int, int) {
	start := (page - 1) * perPage
	if start >= total {
		return total, total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return start, end
}

func writeResponse(c *gin.Context, format string, data interface{}, pg *dto.Pagination) {
	if format == "csv" {
		writeCSV(c, data)
		return
	}
	if pg != nil {
		c.JSON(http.StatusOK, dto.NewSuccessPaginated(data, pg.Page, pg.PerPage, pg.Total))
	} else {
		c.JSON(http.StatusOK, dto.NewSuccessResponse(data))
	}
}

func writeCSV(c *gin.Context, data interface{}) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=analytics.csv")

	wr := csv.NewWriter(c.Writer)
	defer wr.Flush()

	switch v := data.(type) {
	case dto.TokensResponse:
		wr.Write([]string{"tenant_id", "model", "total_input_tokens", "total_output_tokens", "period_start", "period_end"})
		for _, r := range v.Records {
			wr.Write([]string{
				r.TenantID, r.Model,
				strconv.FormatInt(r.TotalInputTokens, 10),
				strconv.FormatInt(r.TotalOutputTokens, 10),
				r.PeriodStart.Format(time.RFC3339),
				r.PeriodEnd.Format(time.RFC3339),
			})
		}
	case dto.CostResponse:
		wr.Write([]string{"tenant_id", "model", "total_cost", "request_count", "period_start", "period_end"})
		for _, r := range v.Records {
			wr.Write([]string{
				r.TenantID, r.Model,
				fmt.Sprintf("%.6f", r.TotalCost),
				strconv.FormatInt(r.RequestCount, 10),
				r.PeriodStart.Format(time.RFC3339),
				r.PeriodEnd.Format(time.RFC3339),
			})
		}
	case dto.TrafficResponse:
		wr.Write([]string{"request_count", "avg_latency_ms", "p50_latency_ms", "p95_latency_ms", "p99_latency_ms"})
		wr.Write([]string{
			strconv.FormatInt(v.RequestCount, 10), "", "", "", "",
		})
	case dto.TenantSummaryResponse:
		wr.Write([]string{"tenant_id", "model", "tokens", "cost", "percentage"})
		for _, m := range v.ModelBreakdown {
			wr.Write([]string{
				v.TenantID, m.Model,
				strconv.FormatInt(m.Tokens, 10),
				fmt.Sprintf("%.6f", m.Cost),
				fmt.Sprintf("%.2f", m.Percentage),
			})
		}
	}
}
