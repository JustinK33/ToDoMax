// Package wellness stores three small, independent daily logs - body
// weight, sleep, and mood. Deliberately simple, mirroring goal.go: no unit
// conversion, no per-day uniqueness/upsert, just plain CRUD ordered by date.
package wellness

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidInput is returned when input fields don't parse or don't satisfy
// a validation rule.
type ErrInvalidInput struct{ Reason string }

func (e ErrInvalidInput) Error() string { return e.Reason }

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func parseLogDate(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, ErrInvalidInput{"log_date must be YYYY-MM-DD"}
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func formatLogDate(d pgtype.Date) string {
	return d.Time.Format("2006-01-02")
}

// BodyMetric is a single weigh-in entry. Weight is unitless, like
// workout_sets.weight - the frontend labels it, no unit conversion.
type BodyMetric struct {
	ID        string    `json:"id"`
	LogDate   string    `json:"log_date"`
	Weight    float64   `json:"weight"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BodyMetricInput struct {
	LogDate string  `json:"log_date"`
	Weight  float64 `json:"weight"`
	Notes   *string `json:"notes"`
}

const bodyMetricColumns = "id, log_date, weight, notes, created_at, updated_at"

func scanBodyMetric(row pgx.Row) (BodyMetric, error) {
	var m BodyMetric
	var logDate pgtype.Date
	err := row.Scan(&m.ID, &logDate, &m.Weight, &m.Notes, &m.CreatedAt, &m.UpdatedAt)
	m.LogDate = formatLogDate(logDate)
	return m, err
}

func validateBodyMetricInput(in BodyMetricInput) (pgtype.Date, error) {
	if in.Weight <= 0 {
		return pgtype.Date{}, ErrInvalidInput{"weight must be positive"}
	}
	return parseLogDate(in.LogDate)
}

func (s *Store) CreateBodyMetric(ctx context.Context, userID string, in BodyMetricInput) (BodyMetric, error) {
	logDate, err := validateBodyMetricInput(in)
	if err != nil {
		return BodyMetric{}, err
	}
	row := s.db.QueryRow(ctx, `
		insert into body_metrics (user_id, log_date, weight, notes)
		values ($1, $2, $3, $4)
		returning `+bodyMetricColumns,
		userID, logDate, in.Weight, in.Notes,
	)
	return scanBodyMetric(row)
}

func (s *Store) ListBodyMetrics(ctx context.Context, userID string) ([]BodyMetric, error) {
	rows, err := s.db.Query(ctx, `
		select `+bodyMetricColumns+`
		from body_metrics
		where user_id = $1
		order by log_date desc, created_at desc`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []BodyMetric
	for rows.Next() {
		m, err := scanBodyMetric(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

func (s *Store) GetBodyMetric(ctx context.Context, userID, id string) (BodyMetric, error) {
	row := s.db.QueryRow(ctx, `
		select `+bodyMetricColumns+`
		from body_metrics
		where user_id = $1 and id = $2`,
		userID, id,
	)
	return scanBodyMetric(row)
}

func (s *Store) UpdateBodyMetric(ctx context.Context, userID, id string, in BodyMetricInput) (BodyMetric, error) {
	logDate, err := validateBodyMetricInput(in)
	if err != nil {
		return BodyMetric{}, err
	}
	row := s.db.QueryRow(ctx, `
		update body_metrics
		set log_date = $3, weight = $4, notes = $5, updated_at = now()
		where user_id = $1 and id = $2
		returning `+bodyMetricColumns,
		userID, id, logDate, in.Weight, in.Notes,
	)
	return scanBodyMetric(row)
}

func (s *Store) DeleteBodyMetric(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from body_metrics where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
