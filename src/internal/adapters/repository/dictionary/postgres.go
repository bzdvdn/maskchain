package dictionaryrepo

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 24-shield-dictionaries#T4.2: Implement PostgresDictionaryRepo (AC-002)
type PostgresDictionaryRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresDictionaryRepo(pool *pgxpool.Pool) *PostgresDictionaryRepo {
	return &PostgresDictionaryRepo{pool: pool}
}

func (r *PostgresDictionaryRepo) Save(ctx context.Context, dict *dictionary.Dictionary) error {
	if r.pool == nil {
		return errors.New("database not available")
	}

	data, err := json.Marshal(dict.Entries())
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO dictionary_entries (profile_slug, name, entries, match_mode, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (profile_slug) DO UPDATE SET
		   name = EXCLUDED.name,
		   entries = EXCLUDED.entries,
		   match_mode = EXCLUDED.match_mode,
		   updated_at = now()`,
		dict.ProfileSlug().String(), dict.Name(), data, string(dict.MatchMode()))
	return err
}

func (r *PostgresDictionaryRepo) FindByProfileSlug(ctx context.Context, slug string) (*dictionary.Dictionary, error) {
	if r.pool == nil {
		return nil, nil
	}

	var name string
	var entriesJSON []byte
	var matchModeStr string

	err := r.pool.QueryRow(ctx,
		`SELECT name, entries, match_mode FROM dictionary_entries WHERE profile_slug = $1`,
		slug).Scan(&name, &entriesJSON, &matchModeStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var entries []string
	if err := json.Unmarshal(entriesJSON, &entries); err != nil {
		return nil, err
	}

	profileSlug, err := value.NewProfileSlug(slug)
	if err != nil {
		return nil, err
	}

	return dictionary.NewDictionary(profileSlug, name, entries, dictionary.MatchMode(matchModeStr)), nil
}

func (r *PostgresDictionaryRepo) Delete(ctx context.Context, slug string) error {
	if r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM dictionary_entries WHERE profile_slug = $1`, slug)
	return err
}
