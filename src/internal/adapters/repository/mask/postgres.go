package maskrepo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/mask"
)

// @sk-task 22-shield-mask-storage#T3.1: Implement PostgresMaskRepo (AC-008, AC-012)
type PostgresMaskRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresMaskRepo(pool *pgxpool.Pool) *PostgresMaskRepo {
	return &PostgresMaskRepo{pool: pool}
}

func (r *PostgresMaskRepo) Save(ctx context.Context, entry *mask.MaskEntry) error {
	if r.pool == nil {
		return mask.ErrMaskNotFound
	}
	data, err := json.Marshal(entry.Replacements)
	if err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx,
		`INSERT INTO mask_entries (mask_id, document_mask_id, profile_id, replacements, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (mask_id) DO NOTHING`,
		entry.MaskID, entry.DocumentMaskID, entry.ProfileID, data, entry.CreatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return mask.ErrMaskIDConflict
	}
	return nil
}

func (r *PostgresMaskRepo) Get(ctx context.Context, maskID string) (*mask.MaskEntry, error) {
	if r.pool == nil {
		return nil, mask.ErrMaskNotFound
	}
	var replacementsJSON []byte
	var profileID *string
	var documentMaskID string
	var createdAt time.Time

	err := r.pool.QueryRow(ctx,
		`SELECT mask_id, document_mask_id, profile_id, replacements, created_at FROM mask_entries WHERE mask_id = $1`,
		maskID).Scan(&maskID, &documentMaskID, &profileID, &replacementsJSON, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, mask.ErrMaskNotFound
		}
		return nil, err
	}

	replacements := make(map[string]string)
	if err := json.Unmarshal(replacementsJSON, &replacements); err != nil {
		return nil, err
	}

	return &mask.MaskEntry{
		MaskID:          maskID,
		DocumentMaskID:  documentMaskID,
		ProfileID:       profileID,
		Replacements:    replacements,
		CreatedAt:       createdAt,
	}, nil
}

func (r *PostgresMaskRepo) Delete(ctx context.Context, maskID string) error {
	if r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM mask_entries WHERE mask_id = $1`, maskID)
	return err
}
