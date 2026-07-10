package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 30-shield-persistence#T3.1: Implement PostgresIncidentRepo (DM-003)
type PostgresIncidentRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresIncidentRepo(pool *pgxpool.Pool) *PostgresIncidentRepo {
	return &PostgresIncidentRepo{pool: pool}
}

func (r *PostgresIncidentRepo) Save(ctx context.Context, incident *entity.Incident) error {
	q := getQuerier(ctx, r.pool)

	_, err := q.Exec(ctx, `
		INSERT INTO incidents (profile_slug, request_id, detector_type, entry_value, severity, action, raw_snippet, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		incident.ProfileSlug(),
		incident.RequestID(),
		incident.DetectorType(),
		incident.EntryValue(),
		incident.Severity().String(),
		incident.Action(),
		incident.RawSnippet(),
		incident.Timestamp(),
	)
	if err != nil {
		return fmt.Errorf("save incident: %w", err)
	}
	return nil
}

func (r *PostgresIncidentRepo) FindByID(ctx context.Context, id string) (*entity.Incident, error) {
	q := getQuerier(ctx, r.pool)

	var (
		profileSlug, requestID, detectorType, severityStr, action string
		entryValue, rawSnippet                                    *string
		ts                                                         time.Time
	)

	err := q.QueryRow(ctx, `
		SELECT profile_slug, request_id, detector_type, entry_value, severity, action, raw_snippet, timestamp
		FROM incidents WHERE id = $1`, id).Scan(
		&profileSlug, &requestID, &detectorType, &entryValue, &severityStr, &action, &rawSnippet, &ts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find incident by id: %w", err)
	}

	return scanIncident(id, profileSlug, requestID, detectorType, entryValue, severityStr, action, rawSnippet, ts)
}

func (r *PostgresIncidentRepo) ListByProfile(ctx context.Context, profileID value.ProfileID) ([]*entity.Incident, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx, `
		SELECT i.id, i.profile_slug, i.request_id, i.detector_type, i.entry_value, i.severity, i.action, i.raw_snippet, i.timestamp
		FROM incidents i
		JOIN profiles p ON p.slug = i.profile_slug
		WHERE p.id = $1
		ORDER BY i.timestamp DESC`, profileID.String())
	if err != nil {
		return nil, fmt.Errorf("list incidents by profile: %w", err)
	}
	defer rows.Close()

	return scanIncidents(rows)
}

func (r *PostgresIncidentRepo) ListByTenant(ctx context.Context, tenantID value.TenantID) ([]*entity.Incident, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx, `
		SELECT i.id, i.profile_slug, i.request_id, i.detector_type, i.entry_value, i.severity, i.action, i.raw_snippet, i.timestamp
		FROM incidents i
		JOIN profiles p ON p.slug = i.profile_slug
		WHERE p.tenant_id = $1
		ORDER BY i.timestamp DESC`, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("list incidents by tenant: %w", err)
	}
	defer rows.Close()

	return scanIncidents(rows)
}

func scanIncidents(rows pgx.Rows) ([]*entity.Incident, error) {
	var incidents []*entity.Incident

	for rows.Next() {
		var (
			id, profileSlug, requestID, detectorType, severityStr, action string
			entryValue, rawSnippet                                         *string
			ts                                                             time.Time
		)

		if err := rows.Scan(&id, &profileSlug, &requestID, &detectorType, &entryValue, &severityStr, &action, &rawSnippet, &ts); err != nil {
			return nil, fmt.Errorf("scan incident row: %w", err)
		}

		inc, err := scanIncident(id, profileSlug, requestID, detectorType, entryValue, severityStr, action, rawSnippet, ts)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, inc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if incidents == nil {
		return []*entity.Incident{}, nil
	}
	return incidents, nil
}

func scanIncident(id, profileSlug, requestID, detectorType string, entryValue *string, severityStr, action string, rawSnippet *string, ts time.Time) (*entity.Incident, error) {
	sev := parseSeverity(severityStr)

	inc := entity.NewAuditIncident(
		id, profileSlug, requestID, detectorType, entryValue, sev, action, rawSnippet, ts,
	)
	return inc, nil
}

func parseSeverity(s string) value.Severity {
	switch s {
	case "low":
		return value.SeverityLow
	case "medium":
		return value.SeverityMedium
	case "high":
		return value.SeverityHigh
	case "critical":
		return value.SeverityCritical
	default:
		return value.SeverityLow
	}
}

func parseInt32(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse int: %w", err)
	}
	return int32(n), nil
}

var _ shield.IncidentRepository = (*PostgresIncidentRepo)(nil)
