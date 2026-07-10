package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 30-shield-persistence#T2.2: Implement PostgresDictionaryRepo (DM-002)
type PostgresDictionaryRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresDictionaryRepo(pool *pgxpool.Pool) *PostgresDictionaryRepo {
	return &PostgresDictionaryRepo{pool: pool}
}

func (r *PostgresDictionaryRepo) Save(ctx context.Context, dict *dictionary.Dictionary) error {
	q := getQuerier(ctx, r.pool)
	slug := dict.ProfileSlug().String()

	_, err := q.Exec(ctx, `DELETE FROM dictionary_entries WHERE profile_slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("delete old dictionary entries: %w", err)
	}

	for _, entry := range dict.Entries() {
		_, err := q.Exec(ctx,
			`INSERT INTO dictionary_entries (profile_slug, entry_value, match_mode) VALUES ($1, $2, $3)`,
			slug, entry, string(dict.MatchMode()))
		if err != nil {
			return fmt.Errorf("insert dictionary entry: %w", err)
		}
	}

	return nil
}

func (r *PostgresDictionaryRepo) FindByProfileSlug(ctx context.Context, slug string) (*dictionary.Dictionary, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx,
		`SELECT entry_value, match_mode FROM dictionary_entries WHERE profile_slug = $1 ORDER BY id`,
		slug)
	if err != nil {
		return nil, fmt.Errorf("query dictionary entries: %w", err)
	}
	defer rows.Close()

	var entries []string
	var matchMode dictionary.MatchMode
	first := true

	for rows.Next() {
		var entryValue string
		var modeStr string
		if err := rows.Scan(&entryValue, &modeStr); err != nil {
			return nil, fmt.Errorf("scan dictionary entry: %w", err)
		}
		entries = append(entries, entryValue)
		if first {
			matchMode = dictionary.MatchMode(modeStr)
			first = false
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	profileSlug, err := value.NewProfileSlug(slug)
	if err != nil {
		return nil, fmt.Errorf("invalid slug: %w", err)
	}

	return dictionary.NewDictionary(profileSlug, "", entries, matchMode), nil
}

func (r *PostgresDictionaryRepo) Delete(ctx context.Context, slug string) error {
	q := getQuerier(ctx, r.pool)
	_, err := q.Exec(ctx, `DELETE FROM dictionary_entries WHERE profile_slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("delete dictionary entries: %w", err)
	}
	return nil
}

var _ dictionary.DictionaryRepository = (*PostgresDictionaryRepo)(nil)
