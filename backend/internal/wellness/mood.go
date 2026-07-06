package wellness

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type MoodLog struct {
	ID        string    `json:"id"`
	LogDate   string    `json:"log_date"`
	Mood      int       `json:"mood"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MoodLogInput struct {
	LogDate string  `json:"log_date"`
	Mood    int     `json:"mood"`
	Notes   *string `json:"notes"`
}

const moodLogColumns = "id, log_date, mood, notes, created_at, updated_at"

func scanMoodLog(row pgx.Row) (MoodLog, error) {
	var m MoodLog
	var logDate pgtype.Date
	err := row.Scan(&m.ID, &logDate, &m.Mood, &m.Notes, &m.CreatedAt, &m.UpdatedAt)
	m.LogDate = formatLogDate(logDate)
	return m, err
}

func validateMoodLogInput(in MoodLogInput) (pgtype.Date, error) {
	if in.Mood < 1 || in.Mood > 5 {
		return pgtype.Date{}, ErrInvalidInput{"mood must be between 1 and 5"}
	}
	return parseLogDate(in.LogDate)
}

func (s *Store) CreateMoodLog(ctx context.Context, userID string, in MoodLogInput) (MoodLog, error) {
	logDate, err := validateMoodLogInput(in)
	if err != nil {
		return MoodLog{}, err
	}
	row := s.db.QueryRow(ctx, `
		insert into mood_logs (user_id, log_date, mood, notes)
		values ($1, $2, $3, $4)
		returning `+moodLogColumns,
		userID, logDate, in.Mood, in.Notes,
	)
	return scanMoodLog(row)
}

func (s *Store) ListMoodLogs(ctx context.Context, userID string) ([]MoodLog, error) {
	rows, err := s.db.Query(ctx, `
		select `+moodLogColumns+`
		from mood_logs
		where user_id = $1
		order by log_date desc, created_at desc`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []MoodLog
	for rows.Next() {
		m, err := scanMoodLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, m)
	}
	return logs, rows.Err()
}

func (s *Store) GetMoodLog(ctx context.Context, userID, id string) (MoodLog, error) {
	row := s.db.QueryRow(ctx, `
		select `+moodLogColumns+`
		from mood_logs
		where user_id = $1 and id = $2`,
		userID, id,
	)
	return scanMoodLog(row)
}

func (s *Store) UpdateMoodLog(ctx context.Context, userID, id string, in MoodLogInput) (MoodLog, error) {
	logDate, err := validateMoodLogInput(in)
	if err != nil {
		return MoodLog{}, err
	}
	row := s.db.QueryRow(ctx, `
		update mood_logs
		set log_date = $3, mood = $4, notes = $5, updated_at = now()
		where user_id = $1 and id = $2
		returning `+moodLogColumns,
		userID, id, logDate, in.Mood, in.Notes,
	)
	return scanMoodLog(row)
}

func (s *Store) DeleteMoodLog(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from mood_logs where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
