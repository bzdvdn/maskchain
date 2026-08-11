package budgetapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task 301-budget-enforcement#T1.3: WebhookNotifier delivers threshold alerts (AC-003)
//
// WebhookNotifier logs the alert and POSTs a JSON payload to a webhook URL.
type WebhookNotifier struct {
	webhookURL string
	client     *http.Client
	log        *slog.Logger
}

func NewWebhookNotifier(webhookURL string, log *slog.Logger) *WebhookNotifier {
	return &WebhookNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
		log:        log,
	}
}

// @sk-task 301-budget-enforcement#T1.3: NotifyBudgetExceeded sends an alert (AC-003)
func (n *WebhookNotifier) NotifyBudgetExceeded(ctx context.Context, b *budget.Budget, pct float64, spent float64) error {
	n.log.WarnContext(ctx, "budget alert: notify_at threshold crossed",
		"budget_id", b.ID,
		"tenant_id", b.TenantID,
		"scope", string(b.Scope),
		"threshold_pct", pct,
		"spent", spent,
	)
	if n.webhookURL == "" {
		return nil
	}

	payload := map[string]any{
		"event":         "budget.notify",
		"budget_id":     b.ID,
		"tenant_id":     b.TenantID,
		"scope":         string(b.Scope),
		"threshold_pct": pct,
		"spent":         spent,
		"hard_limit":    b.HardLimit,
		"currency":      b.Currency,
		"notified_at":   time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal budget alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create budget alert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send budget alert: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("budget alert webhook returned %d", resp.StatusCode)
	}
	return nil
}
