package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/budget"
)

// @sk-task 301-budget-enforcement#T1.2: PostgresBudgetRepository (AC-002)
//
// PostgresBudgetRepository persists budgets and spend history.
type PostgresBudgetRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresBudgetRepository(pool *pgxpool.Pool) *PostgresBudgetRepository {
	return &PostgresBudgetRepository{pool: pool}
}

const budgetSelect = `
	SELECT id, tenant_id, virtual_key_id, model, scope, type, custom_days,
	       soft_limit, hard_limit, currency, notify_at, enabled, created_at, updated_at
	FROM budgets`

// @sk-task 301-budget-enforcement#T1.2: Create persists a new budget (AC-002)
func (r *PostgresBudgetRepository) Create(ctx context.Context, b *budget.Budget) error {
	notify, err := json.Marshal(b.NotifyAt)
	if err != nil {
		return fmt.Errorf("marshal notify_at: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO budgets (id, tenant_id, virtual_key_id, model, scope, type, custom_days,
		                     soft_limit, hard_limit, currency, notify_at, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		b.ID, b.TenantID, nullableString(b.VirtualKeyID), b.Model, string(b.Scope), string(b.Type),
		b.CustomDays, b.SoftLimit, b.HardLimit, b.Currency, notify, b.Enabled,
		b.CreatedAt, b.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create budget: %w", err)
	}
	return nil
}

// @sk-task 301-budget-enforcement#T1.2: Update persists changes to an existing budget (AC-002)
func (r *PostgresBudgetRepository) Update(ctx context.Context, b *budget.Budget) error {
	notify, err := json.Marshal(b.NotifyAt)
	if err != nil {
		return fmt.Errorf("marshal notify_at: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE budgets
		SET virtual_key_id = $2, model = $3, scope = $4, type = $5, custom_days = $6,
		    soft_limit = $7, hard_limit = $8, currency = $9, notify_at = $10, enabled = $11, updated_at = $12
		WHERE id = $1`,
		b.ID, nullableString(b.VirtualKeyID), b.Model, string(b.Scope), string(b.Type),
		b.CustomDays, b.SoftLimit, b.HardLimit, b.Currency, notify, b.Enabled, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return budget.ErrNotFound
	}
	return nil
}

// @sk-task 301-budget-enforcement#T1.2: Delete removes a budget (AC-002)
func (r *PostgresBudgetRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM budgets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return budget.ErrNotFound
	}
	return nil
}

// @sk-task 301-budget-enforcement#T1.2: GetByID returns a budget by id (AC-002)
func (r *PostgresBudgetRepository) GetByID(ctx context.Context, id string) (*budget.Budget, error) {
	row := r.pool.QueryRow(ctx, budgetSelect+` WHERE id = $1`, id)
	return scanBudget(row)
}

// @sk-task 301-budget-enforcement#T1.2: List returns all budgets (AC-002)
func (r *PostgresBudgetRepository) List(ctx context.Context) ([]*budget.Budget, error) {
	rows, err := r.pool.Query(ctx, budgetSelect+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()
	return collectBudgets(rows)
}

// @sk-task 301-budget-enforcement#T1.2: ListByTenant returns budgets of a tenant (AC-002)
func (r *PostgresBudgetRepository) ListByTenant(ctx context.Context, tenantID string) ([]*budget.Budget, error) {
	rows, err := r.pool.Query(ctx, budgetSelect+` WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant budgets: %w", err)
	}
	defer rows.Close()
	return collectBudgets(rows)
}

// @sk-task 301-budget-enforcement#T1.2: ListActiveForRequest returns enabled matching budgets (AC-002)
func (r *PostgresBudgetRepository) ListActiveForRequest(ctx context.Context, tenantID, keyID, model string) ([]*budget.Budget, error) {
	rows, err := r.pool.Query(ctx, budgetSelect+`
		WHERE enabled = TRUE
		  AND tenant_id = $1
		  AND (
		    (scope = 'tenant' AND virtual_key_id IS NULL)
		    OR (scope = 'key' AND virtual_key_id = $2 AND $2 <> '')
		    OR (scope = 'model' AND model = $3 AND $3 <> '')
		  )
		ORDER BY created_at DESC`, tenantID, keyID, model)
	if err != nil {
		return nil, fmt.Errorf("list active budgets: %w", err)
	}
	defer rows.Close()
	return collectBudgets(rows)
}

// @sk-task 301-budget-enforcement#T1.2: RecordSpend appends a spend entry (AC-002)
func (r *PostgresBudgetRepository) RecordSpend(ctx context.Context, entry budget.SpendEntry) error {
	id := entry.ID
	if id == "" {
		id = uuid.NewString()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO budget_spend (id, budget_id, virtual_key_id, tenant_id, model, cost, tokens, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, entry.BudgetID, entry.VirtualKeyID, entry.TenantID, entry.Model,
		entry.Cost, entry.Tokens, entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("record budget spend: %w", err)
	}
	return nil
}

// @sk-task 301-budget-enforcement#T1.2: History returns spend entries, newest first (AC-002)
func (r *PostgresBudgetRepository) History(ctx context.Context, budgetID string, limit, offset int) ([]budget.SpendEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, budget_id, virtual_key_id, tenant_id, model, cost, tokens, created_at
		FROM budget_spend
		WHERE budget_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, budgetID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("budget history: %w", err)
	}
	defer rows.Close()
	var out []budget.SpendEntry
	for rows.Next() {
		var e budget.SpendEntry
		if err := rows.Scan(&e.ID, &e.BudgetID, &e.VirtualKeyID, &e.TenantID, &e.Model,
			&e.Cost, &e.Tokens, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan budget spend: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if out == nil {
		return []budget.SpendEntry{}, nil
	}
	return out, nil
}

// @sk-task 301-budget-enforcement#T1.2: AggregateDaily upserts materialized daily spend (AC-002)
func (r *PostgresBudgetRepository) AggregateDaily(ctx context.Context, day time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO budget_spend_daily (budget_id, virtual_key_id, tenant_id, model, day,
		                                total_cost, total_tokens, request_count, updated_at)
		SELECT budget_id, virtual_key_id, tenant_id, model, created_at::date,
		       SUM(cost), SUM(tokens), COUNT(*), NOW()
		FROM budget_spend
		WHERE created_at >= $1 AND created_at < $1 + interval '1 day'
		GROUP BY budget_id, virtual_key_id, tenant_id, model, created_at::date
		ON CONFLICT (budget_id, virtual_key_id, tenant_id, model, day) DO UPDATE SET
			total_cost = EXCLUDED.total_cost,
			total_tokens = EXCLUDED.total_tokens,
			request_count = EXCLUDED.request_count,
			updated_at = NOW()`, day)
	return err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func collectBudgets(rows pgx.Rows) ([]*budget.Budget, error) {
	var out []*budget.Budget
	for rows.Next() {
		b, err := scanBudget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if out == nil {
		return []*budget.Budget{}, nil
	}
	return out, nil
}

func scanBudget(row interface{ Scan(dest ...any) error }) (*budget.Budget, error) {
	var (
		id, tenantID string
		vkID, model  *string
		scope, typ   string
		customDays   int
		soft, hard   *float64
		currency     string
		notifyRaw    []byte
		enabled      bool
		created, up  time.Time
	)
	err := row.Scan(&id, &tenantID, &vkID, &model, &scope, &typ, &customDays,
		&soft, &hard, &currency, &notifyRaw, &enabled, &created, &up)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, budget.ErrNotFound
		}
		return nil, fmt.Errorf("scan budget: %w", err)
	}

	var notify []float64
	if len(notifyRaw) > 0 {
		if err := json.Unmarshal(notifyRaw, &notify); err != nil {
			return nil, fmt.Errorf("unmarshal notify_at: %w", err)
		}
	}
	if notify == nil {
		notify = []float64{}
	}

	b := &budget.Budget{
		ID:           id,
		TenantID:     tenantID,
		VirtualKeyID: derefString(vkID),
		Model:        derefString(model),
		Scope:        budget.Scope(scope),
		Type:         budget.PeriodType(typ),
		CustomDays:   customDays,
		SoftLimit:    soft,
		HardLimit:    hard,
		Currency:     currency,
		NotifyAt:     notify,
		Enabled:      enabled,
		CreatedAt:    created,
		UpdatedAt:    up,
	}
	return b, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var _ budget.BudgetRepository = (*PostgresBudgetRepository)(nil)
