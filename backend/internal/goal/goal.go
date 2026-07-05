// Package goal stores a user's personal goals, bucketed by timeframe
// (week/month/year/lifetime). Deliberately simple - no progress percentages
// or history, just title/notes/timeframe/target_date/completed. Goals are
// optional: a user with zero goals is a normal, expected state.
package goal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Goal is the API-facing representation of a personal goal.
type Goal struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Notes      *string   `json:"notes"`
	Timeframe  string    `json:"timeframe"`
	TargetDate *string   `json:"target_date"`
	Completed  bool      `json:"completed"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Input is the subset of Goal fields a client may set on create/update.
type Input struct {
	Title      string  `json:"title"`
	Notes      *string `json:"notes"`
	Timeframe  string  `json:"timeframe"`
	TargetDate *string `json:"target_date"`
}

// ErrInvalidInput is returned when input fields don't parse or don't satisfy
// a validation rule.
type ErrInvalidInput struct{ Reason string }

func (e ErrInvalidInput) Error() string { return e.Reason }

var validTimeframes = map[string]bool{"week": true, "month": true, "year": true, "lifetime": true}

func parseTargetDate(s *string) (pgtype.Date, error) {
	if s == nil {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, ErrInvalidInput{"target_date must be YYYY-MM-DD"}
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func validateInput(in Input) (pgtype.Date, error) {
	if !validTimeframes[in.Timeframe] {
		return pgtype.Date{}, ErrInvalidInput{"timeframe must be week, month, year, or lifetime"}
	}
	return parseTargetDate(in.TargetDate)
}

func formatDate(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

const goalColumns = "id, title, notes, timeframe, target_date, completed, created_at, updated_at"

func scanGoal(row pgx.Row) (Goal, error) {
	var g Goal
	var targetDate pgtype.Date
	err := row.Scan(&g.ID, &g.Title, &g.Notes, &g.Timeframe, &targetDate, &g.Completed, &g.CreatedAt, &g.UpdatedAt)
	g.TargetDate = formatDate(targetDate)
	return g, err
}

func (s *Store) Create(ctx context.Context, userID string, in Input) (Goal, error) {
	targetDate, err := validateInput(in)
	if err != nil {
		return Goal{}, err
	}

	row := s.db.QueryRow(ctx, `
		insert into goals (user_id, title, notes, timeframe, target_date)
		values ($1, $2, $3, $4, $5)
		returning `+goalColumns,
		userID, in.Title, in.Notes, in.Timeframe, targetDate,
	)
	return scanGoal(row)
}

// List returns all of a user's goals, ordered by timeframe then created_at.
// The frontend buckets by timeframe client-side, the same way it already
// buckets tasks by category.
func (s *Store) List(ctx context.Context, userID string) ([]Goal, error) {
	rows, err := s.db.Query(ctx, `
		select `+goalColumns+` from goals
		where user_id = $1
		order by array_position(array['week','month','year','lifetime'], timeframe), created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []Goal
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (s *Store) Get(ctx context.Context, userID, id string) (Goal, error) {
	row := s.db.QueryRow(ctx, `
		select `+goalColumns+`
		from goals
		where user_id = $1 and id = $2`,
		userID, id,
	)
	return scanGoal(row)
}

func (s *Store) Update(ctx context.Context, userID, id string, in Input) (Goal, error) {
	targetDate, err := validateInput(in)
	if err != nil {
		return Goal{}, err
	}

	row := s.db.QueryRow(ctx, `
		update goals
		set title = $3, notes = $4, timeframe = $5, target_date = $6, updated_at = now()
		where user_id = $1 and id = $2
		returning `+goalColumns,
		userID, id, in.Title, in.Notes, in.Timeframe, targetDate,
	)
	return scanGoal(row)
}

func (s *Store) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from goals where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) SetCompleted(ctx context.Context, userID, id string, completed bool) (Goal, error) {
	row := s.db.QueryRow(ctx, `
		update goals
		set completed = $3, updated_at = now()
		where user_id = $1 and id = $2
		returning `+goalColumns,
		userID, id, completed,
	)
	return scanGoal(row)
}
