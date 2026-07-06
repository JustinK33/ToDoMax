package wellness

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SleepLog struct {
	ID        string    `json:"id"`
	LogDate   string    `json:"log_date"`
	Hours     float64   `json:"hours"`
	Quality   *int      `json:"quality"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SleepLogInput struct {
	LogDate string  `json:"log_date"`
	Hours   float64 `json:"hours"`
	Quality *int    `json:"quality"`
	Notes   *string `json:"notes"`
}

const sleepLogColumns = "id, log_date, hours, quality, notes, created_at, updated_at"

func scanSleepLog(row pgx.Row) (SleepLog, error) {
	var l SleepLog
	var logDate pgtype.Date
	err := row.Scan(&l.ID, &logDate, &l.Hours, &l.Quality, &l.Notes, &l.CreatedAt, &l.UpdatedAt)
	l.LogDate = formatLogDate(logDate)
	return l, err
}

func validateSleepLogInput(in SleepLogInput) (pgtype.Date, error) {
	if in.Hours < 0 || in.Hours > 24 {
		return pgtype.Date{}, ErrInvalidInput{"hours must be between 0 and 24"}
	}
	if in.Quality != nil && (*in.Quality < 1 || *in.Quality > 5) {
		return pgtype.Date{}, ErrInvalidInput{"quality must be between 1 and 5"}
	}
	return parseLogDate(in.LogDate)
}

func (s *Store) CreateSleepLog(ctx context.Context, userID string, in SleepLogInput) (SleepLog, error) {
	logDate, err := validateSleepLogInput(in)
	if err != nil {
		return SleepLog{}, err
	}
	row := s.db.QueryRow(ctx, `
		insert into sleep_logs (user_id, log_date, hours, quality, notes)
		values ($1, $2, $3, $4, $5)
		returning `+sleepLogColumns,
		userID, logDate, in.Hours, in.Quality, in.Notes,
	)
	return scanSleepLog(row)
}

func (s *Store) ListSleepLogs(ctx context.Context, userID string) ([]SleepLog, error) {
	rows, err := s.db.Query(ctx, `
		select `+sleepLogColumns+`
		from sleep_logs
		where user_id = $1
		order by log_date desc, created_at desc`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []SleepLog
	for rows.Next() {
		l, err := scanSleepLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *Store) GetSleepLog(ctx context.Context, userID, id string) (SleepLog, error) {
	row := s.db.QueryRow(ctx, `
		select `+sleepLogColumns+`
		from sleep_logs
		where user_id = $1 and id = $2`,
		userID, id,
	)
	return scanSleepLog(row)
}

func (s *Store) UpdateSleepLog(ctx context.Context, userID, id string, in SleepLogInput) (SleepLog, error) {
	logDate, err := validateSleepLogInput(in)
	if err != nil {
		return SleepLog{}, err
	}
	row := s.db.QueryRow(ctx, `
		update sleep_logs
		set log_date = $3, hours = $4, quality = $5, notes = $6, updated_at = now()
		where user_id = $1 and id = $2
		returning `+sleepLogColumns,
		userID, id, logDate, in.Hours, in.Quality, in.Notes,
	)
	return scanSleepLog(row)
}

func (s *Store) DeleteSleepLog(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from sleep_logs where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
