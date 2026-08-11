package analyticsrepo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/analytics"
)

// @sk-task 150-admin-routing-crud#T1.3: PostgresCostRateStore (AC-002)
//
// PostgresCostRateStore persists per-model cost rates with currency and provenance.
type PostgresCostRateStore struct {
	pool *pgxpool.Pool
}

func NewPostgresCostRateStore(pool *pgxpool.Pool) *PostgresCostRateStore {
	return &PostgresCostRateStore{pool: pool}
}

func (s *PostgresCostRateStore) List(ctx context.Context) ([]*analytics.CostRate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model, input_price_per_1k, output_price_per_1k, currency, source
		FROM cost_rates
		ORDER BY model`)
	if err != nil {
		return nil, fmt.Errorf("list cost rates: %w", err)
	}
	defer rows.Close()

	var out []*analytics.CostRate
	for rows.Next() {
		var model, currency, source string
		var in, outP float64
		if err := rows.Scan(&model, &in, &outP, &currency, &source); err != nil {
			return nil, err
		}
		out = append(out, &analytics.CostRate{
			Model:            model,
			InputPricePer1K:  in,
			OutputPricePer1K: outP,
			Currency:         currency,
			Source:           source,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if out == nil {
		return []*analytics.CostRate{}, nil
	}
	return out, nil
}

func (s *PostgresCostRateStore) Upsert(ctx context.Context, rate *analytics.CostRate) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cost_rates (model, input_price_per_1k, output_price_per_1k, currency, source)
		VALUES ($1, $2, $3, $4, 'ui')
		ON CONFLICT (model) DO UPDATE SET
			input_price_per_1k = EXCLUDED.input_price_per_1k,
			output_price_per_1k = EXCLUDED.output_price_per_1k,
			currency = EXCLUDED.currency,
			source = 'ui',
			updated_at = now()`,
		rate.Model, rate.InputPricePer1K, rate.OutputPricePer1K, rate.Currency)
	if err != nil {
		return fmt.Errorf("upsert cost rate %s: %w", rate.Model, err)
	}
	return nil
}

func (s *PostgresCostRateStore) Delete(ctx context.Context, model string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM cost_rates WHERE model = $1`, model)
	if err != nil {
		return fmt.Errorf("delete cost rate %s: %w", model, err)
	}
	if tag.RowsAffected() == 0 {
		return analytics.ErrNotFound
	}
	return nil
}

func (s *PostgresCostRateStore) SeedFromYAML(ctx context.Context, rates []*analytics.CostRate) (bool, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM cost_rates`).Scan(&count); err != nil {
		return false, fmt.Errorf("count cost rates: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	for _, rate := range rates {
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO cost_rates (model, input_price_per_1k, output_price_per_1k, currency, source)
			VALUES ($1, $2, $3, $4, 'yaml')`,
			rate.Model, rate.InputPricePer1K, rate.OutputPricePer1K, rate.Currency); err != nil {
			return false, fmt.Errorf("seed cost rate %s: %w", rate.Model, err)
		}
	}
	return true, nil
}
