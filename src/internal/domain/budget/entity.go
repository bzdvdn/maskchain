package budget

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a budget does not exist.
var ErrNotFound = errors.New("budget not found")

// DefaultCurrency is applied when a budget has no explicit currency set.
const DefaultCurrency = "USD"

// Scope defines what a budget bounds.
type Scope string

const (
	ScopeTenant Scope = "tenant"
	ScopeKey    Scope = "key"
	ScopeModel  Scope = "model"
)

// PeriodType defines the budget window.
type PeriodType string

const (
	PeriodMonthly PeriodType = "monthly"
	PeriodDaily   PeriodType = "daily"
	PeriodCustom  PeriodType = "custom"
)

// @sk-task 301-budget-enforcement#T1.1: Budget entity with scope and period (AC-001)
//
// Budget bounds spend for a tenant, a virtual key, or a model over a window.
type Budget struct {
	ID           string
	TenantID     string
	VirtualKeyID string
	Model        string
	Scope        Scope
	Type         PeriodType
	CustomDays   int
	SoftLimit    *float64
	HardLimit    *float64
	Currency     string
	NotifyAt     []float64
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// @sk-task 301-budget-enforcement#T1.1: NewBudgetID generates a random budget id (AC-001)
func NewBudgetID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate budget id: %w", err)
	}
	return "budget_" + hex.EncodeToString(b), nil
}

// @sk-task 301-budget-enforcement#T1.1: NewBudget validates and constructs a budget (AC-001)
func NewBudget(id, tenantID string, scope Scope, period PeriodType) (*Budget, error) {
	if id == "" {
		return nil, fmt.Errorf("id must not be empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id must not be empty")
	}
	switch scope {
	case ScopeTenant, ScopeKey, ScopeModel:
	default:
		return nil, fmt.Errorf("scope must be one of tenant|key|model")
	}
	switch period {
	case PeriodMonthly, PeriodDaily, PeriodCustom:
	default:
		return nil, fmt.Errorf("type must be one of monthly|daily|custom")
	}
	if scope == ScopeKey {
		if tenantID == "" {
			return nil, fmt.Errorf("key scope requires a key id")
		}
	}
	now := time.Now().UTC()
	return &Budget{
		ID:        id,
		TenantID:  tenantID,
		Scope:     scope,
		Type:      period,
		Currency:  DefaultCurrency,
		NotifyAt:  []float64{},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// @sk-task 301-budget-enforcement#T1.1: PeriodKey returns the current period bucket for the budget type (AC-001)
func (b *Budget) PeriodKey(now time.Time) string {
	now = now.UTC()
	switch b.Type {
	case PeriodMonthly:
		return now.Format("2006-01")
	case PeriodDaily:
		return now.Format("2006-01-02")
	case PeriodCustom:
		days := b.CustomDays
		if days <= 0 {
			days = 30
		}
		epochDays := now.Unix() / 86400
		window := epochDays / int64(days) * int64(days)
		return time.Unix(window*86400, 0).UTC().Format("2006-01-02")
	}
	return now.Format("2006-01-02")
}

// @sk-task 301-budget-enforcement#T1.1: CounterKey returns the Valkey counter key for the current period (AC-001)
func (b *Budget) CounterKey(now time.Time) string {
	entity := b.TenantID
	switch b.Scope {
	case ScopeKey:
		entity = b.VirtualKeyID
	case ScopeModel:
		entity = b.TenantID + ":" + b.Model
	}
	return KeyPrefixBudget + string(b.Scope) + ":" + entity + ":" + b.PeriodKey(now)
}

// @sk-task 301-budget-enforcement#T1.1: PeriodTTL returns time remaining in the current period (AC-001)
func (b *Budget) PeriodTTL(now time.Time) time.Duration {
	now = now.UTC()
	switch b.Type {
	case PeriodMonthly:
		next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		return next.Sub(now)
	case PeriodDaily:
		next := now.AddDate(0, 0, 1)
		next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
		return next.Sub(now)
	case PeriodCustom:
		days := b.CustomDays
		if days <= 0 {
			days = 30
		}
		epochDays := now.Unix() / 86400
		window := epochDays / int64(days) * int64(days)
		end := time.Unix((window+int64(days))*86400, 0).UTC()
		return end.Sub(now)
	}
	return 24 * time.Hour
}

// @sk-task 301-budget-enforcement#T1.1: HardExceeded reports whether spent reached the hard limit (AC-001)
func (b *Budget) HardExceeded(spent float64) bool {
	return b.HardLimit != nil && spent >= *b.HardLimit
}

// @sk-task 301-budget-enforcement#T1.1: SoftExceeded reports whether spent reached the soft limit (AC-001)
func (b *Budget) SoftExceeded(spent float64) bool {
	return b.SoftLimit != nil && spent >= *b.SoftLimit
}

// @sk-task 301-budget-enforcement#T1.1: CrossedThresholds returns notify_at percentages crossed between two spends (AC-001)
func (b *Budget) CrossedThresholds(prevSpent, newSpent float64) []float64 {
	if b.HardLimit == nil || *b.HardLimit <= 0 {
		return nil
	}
	var crossed []float64
	for _, pct := range b.NotifyAt {
		if pct < 0 || pct > 100 {
			continue
		}
		threshold := *b.HardLimit * pct / 100
		if prevSpent < threshold && newSpent >= threshold {
			crossed = append(crossed, pct)
		}
	}
	return crossed
}
